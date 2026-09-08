package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"code-common/backend/auth"
	"code-gate/internal/models"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ContextUserID    = "user_id"
	ContextAPIKeyID  = "api_key_id"
	ContextUserQuota = "user_quota"
	ContextWallet    = "credits_wallet"
)

// CachedKeyInfo API Key 内存缓存结构体
type CachedKeyInfo struct {
	KeyID     uint
	UserID    uint
	ExpiresAt *time.Time
	IsActive  bool
	CachedAt  time.Time
}

var (
	keyCache     = make(map[string]CachedKeyInfo)
	keyCacheLock sync.RWMutex
)

// HashAPIKey 计算 API Key 的 SHA-256 哈希值
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// UnifiedAuthMiddleware 统一凭证认证中间件：支持 API Key (sk-...) 与 JWT 混合认证
func UnifiedAuthMiddleware(jwtSecretGetter func() string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := auth.ExtractToken(c)
		if tokenStr == "" {
			if cookie, err := c.Cookie(auth.StandardTokenKey); err == nil && cookie != "" {
				tokenStr = cookie
			}
		}
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "缺少 Authorization 认证标头或凭证",
					"type":    "invalid_request_error",
					"code":    "unauthorized",
				},
			})
			return
		}

		db := store.GetDB()
		if db == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "数据库未初始化",
					"type":    "internal_error",
				},
			})
			return
		}

		// 判定凭证类型：若是 sk- 前缀，走 API Key 校验
		if strings.HasPrefix(tokenStr, "sk-") {
			keyHash := HashAPIKey(tokenStr)

			// 1. 尝试从本地缓存读取
			keyCacheLock.RLock()
			cached, ok := keyCache[keyHash]
			keyCacheLock.RUnlock()

			if ok && time.Since(cached.CachedAt) < 5*time.Minute {
				if !cached.IsActive || (cached.ExpiresAt != nil && time.Now().After(*cached.ExpiresAt)) {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error": gin.H{
							"message": "API Key 已被停用或已过期",
							"type":    "invalid_api_key",
							"code":    "invalid_api_key",
						},
					})
					return
				}
				attachUserAndQuota(c, db, cached.UserID, &cached.KeyID)
				c.Next()
				return
			}

			// 2. 查库校验
			var apiKey models.APIKey
			if err := db.Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"message": "无效或未授权的 API Key",
						"type":    "invalid_api_key",
						"code":    "invalid_api_key",
					},
				})
				return
			}

			if !apiKey.IsActive || (apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt)) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"message": "API Key 已被停用或已过期",
						"type":    "invalid_api_key",
						"code":    "invalid_api_key",
					},
				})
				return
			}

			// 写入内存缓存
			keyCacheLock.Lock()
			keyCache[keyHash] = CachedKeyInfo{
				KeyID:     apiKey.ID,
				UserID:    apiKey.UserID,
				ExpiresAt: apiKey.ExpiresAt,
				IsActive:  apiKey.IsActive,
				CachedAt:  time.Now(),
			}
			keyCacheLock.Unlock()

			attachUserAndQuota(c, db, apiKey.UserID, &apiKey.ID)
			c.Next()
			return
		}

		// 否则走标准 JWT 校验流程（CodeBench 登录用户 SSO）
		claims, err := auth.ParseToken(tokenStr, jwtSecretGetter())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "无效或已过期的登录凭证: " + err.Error(),
					"type":    "invalid_request_error",
					"code":    "unauthorized",
				},
			})
			return
		}

		// 注入统一规范身份上下文
		c.Set(auth.ContextClaims, claims)
		c.Set(auth.ContextUserID, claims.UserID)
		c.Set(auth.ContextUsername, claims.Username)
		c.Set(auth.ContextEmail, claims.Email)
		c.Set(auth.ContextName, claims.Name)
		c.Set(auth.ContextEmployeeID, claims.EmployeeID)
		c.Set(auth.ContextIsAdmin, claims.IsAdmin)
		c.Set(auth.ContextRoles, claims.Roles)

		// 绑定网关用户 ID、配额角色与钱包资产
		attachUserAndQuota(c, db, claims.UserID, nil)

		c.Next()
	}
}

// attachUserAndQuota 绑定用户 ID、配额角色与钱包至 Context
func attachUserAndQuota(c *gin.Context, db *gorm.DB, userID uint, keyID *uint) {
	c.Set(ContextUserID, userID)
	if keyID != nil {
		c.Set(ContextAPIKeyID, *keyID)
	}

	// 判定是否具备平台超级管理员/网关管理员权限
	isAdmin := false
	if rawAdmin, exists := c.Get(auth.ContextIsAdmin); exists {
		if a, ok := rawAdmin.(bool); ok && a {
			isAdmin = true
		}
	}
	if !isAdmin {
		if rawRoles, exists := c.Get(auth.ContextRoles); exists {
			if roles, ok := rawRoles.([]string); ok {
				for _, r := range roles {
					if r == "admin" || r == "super_admin" || r == "gate_admin" {
						isAdmin = true
						break
					}
				}
			}
		}
	}

	var quota *models.GateUserQuota
	var wallet *models.CreditsWallet
	var err error

	if isAdmin {
		quota, err = store.EnsureAdminQuota(db, userID)
		if err == nil {
			_, wallet, _ = store.InitOrGetUserQuota(db, userID)
		}
	} else {
		quota, wallet, err = store.InitOrGetUserQuota(db, userID)
	}

	if err == nil && quota != nil {
		c.Set(ContextUserQuota, quota)
		if wallet != nil {
			c.Set(ContextWallet, wallet)
		}
	}
}
