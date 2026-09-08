package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code-common/backend/auth"
	"code-gate/internal/models"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// UserProfileResponse 用户个人网关资产与配额概览
type UserProfileResponse struct {
	UserID                 uint    `json:"user_id"`
	Role                   string  `json:"role"`
	PolicyName             string  `json:"policy_name"`
	IsCustom               bool    `json:"is_custom"`
	IsAdmin                bool    `json:"is_admin"`
	RPMLimit               int     `json:"rpm_limit"`
	DailyLimitCredits      float64 `json:"daily_limit_credits"`
	WeeklyLimitCredits     float64 `json:"weekly_limit_credits"`
	DailyUsedCredits       float64 `json:"daily_used_credits"`
	WeeklyUsedCredits      float64 `json:"weekly_used_credits"`
	DailyRemainingCredits  float64 `json:"daily_remaining_credits"`
	WeeklyRemainingCredits float64 `json:"weekly_remaining_credits"`
}

// HandleGetUserProfile 获取当前登录用户的配额与资产概览
func HandleGetUserProfile(c *gin.Context) {
	rawUID, exists := c.Get(ContextUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未获取到有效的用户身份"})
		return
	}
	userID := rawUID.(uint)

	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接异常"})
		return
	}

	// 判定当前用户是否为超级管理员/管理员
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

	var userQuota *models.GateUserQuota
	var wallet *models.CreditsWallet
	var err error

	if isAdmin {
		userQuota, err = store.EnsureAdminQuota(db, userID)
		if err == nil {
			_, wallet, _ = store.InitOrGetUserQuota(db, userID)
		}
	} else {
		userQuota, wallet, err = store.InitOrGetUserQuota(db, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户配额失败: " + err.Error()})
		return
	}

	if !isAdmin && userQuota.Role == models.RoleAdmin {
		isAdmin = true
	}

	policyName := "guest_policy"
	rpmLimit := 60
	dailyLimit := 100.0
	weeklyLimit := 400.0
	isCustom := false

	if userQuota.Policy != nil {
		policyName = userQuota.Policy.Name
		rpmLimit = userQuota.Policy.RateLimitRPM
		dailyLimit = userQuota.Policy.DailyCreditsLimit
		weeklyLimit = userQuota.Policy.WeeklyCreditsLimit
	}

	if userQuota.CustomDailyCredits != nil {
		dailyLimit = *userQuota.CustomDailyCredits
		isCustom = true
	}
	if userQuota.CustomWeeklyCredits != nil {
		weeklyLimit = *userQuota.CustomWeeklyCredits
		isCustom = true
	}

	dailyRemaining := dailyLimit - wallet.DailyConsumed
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}
	weeklyRemaining := weeklyLimit - wallet.WeeklyConsumed
	if weeklyRemaining < 0 {
		weeklyRemaining = 0
	}

	resp := UserProfileResponse{
		UserID:                 userID,
		Role:                   userQuota.Role,
		PolicyName:             policyName,
		IsCustom:               isCustom,
		IsAdmin:                isAdmin,
		RPMLimit:               rpmLimit,
		DailyLimitCredits:      dailyLimit,
		WeeklyLimitCredits:     weeklyLimit,
		DailyUsedCredits:       wallet.DailyConsumed,
		WeeklyUsedCredits:      wallet.WeeklyConsumed,
		DailyRemainingCredits:  dailyRemaining,
		WeeklyRemainingCredits: weeklyRemaining,
	}

	c.JSON(http.StatusOK, gin.H{
		"data": resp,
	})
}

// HandleListUserKeys 查询当前登录用户的 API Key 列表
func HandleListUserKeys(c *gin.Context) {
	rawUID, exists := c.Get(ContextUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未获取到有效用户身份"})
		return
	}
	userID := rawUID.(uint)

	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接异常"})
		return
	}

	var keys []models.APIKey
	if err := db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询 API Key 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": keys})
}

// CreateKeyRequest 创建 Key 请求体
type CreateKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresIn int    `json:"expires_in"` // 过期天数，0 表示永不过期
}

// HandleCreateUserKey 为当前用户新建 API Key
func HandleCreateUserKey(c *gin.Context) {
	rawUID, exists := c.Get(ContextUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未获取到有效用户身份"})
		return
	}
	userID := rawUID.(uint)

	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法: " + err.Error()})
		return
	}

	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接异常"})
		return
	}

	// 限制单个用户最多创建 20 个 Key
	var count int64
	db.Model(&models.APIKey{}).Where("user_id = ?", userID).Count(&count)
	if count >= 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单个用户最多创建 20 个 API Key"})
		return
	}

	// 生成安全高随机度 Key: sk-gate- + 32位 Hex (16 bytes)
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成随机密钥失败"})
		return
	}
	rawKey := fmt.Sprintf("sk-gate-%s", hex.EncodeToString(randomBytes))
	keyHash := HashAPIKey(rawKey)
	keyPrefix := rawKey[:12] + "..."

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		expiresAt = &t
	}

	apiKey := models.APIKey{
		UserID:        userID,
		Name:          req.Name,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		AllowedModels: datatypes.JSON([]byte(`["*"]`)),
		ExpiresAt:     expiresAt,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := db.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 API Key 失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API Key 创建成功，请妥善保存（明文仅展示一次）",
		"data": gin.H{
			"id":         apiKey.ID,
			"name":       apiKey.Name,
			"key_prefix": apiKey.KeyPrefix,
			"raw_key":    rawKey,
			"expires_at": apiKey.ExpiresAt,
			"created_at": apiKey.CreatedAt,
		},
	})
}

// HandleDeleteUserKey 删除当前用户的 API Key
func HandleDeleteUserKey(c *gin.Context) {
	rawUID, exists := c.Get(ContextUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未获取到有效用户身份"})
		return
	}
	userID := rawUID.(uint)

	keyIDStr := c.Param("id")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 Key ID"})
		return
	}

	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库连接异常"})
		return
	}

	res := db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&models.APIKey{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除 API Key 失败"})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到对应的 API Key 或无权操作"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API Key 删除成功"})
}
