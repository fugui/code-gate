package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"code-common/backend/auth"
	commonModels "code-common/backend/models"
	"code-gate/internal/models"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// RequireAdmin 权限中间件：确保只有管理员方可访问
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
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
						if r == "admin" || r == "super_admin" {
							isAdmin = true
							break
						}
					}
				}
			}
		}

		if !isAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "权限不足：需要管理员权限",
					"type":    "permission_denied",
				},
			})
			return
		}

		c.Next()
	}
}

// UserQuotaDetailDTO 用户及其配额消耗视图
type UserQuotaDetailDTO struct {
	UserID              uint     `json:"user_id"`
	Username            string   `json:"username"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	Role                string   `json:"role"` // CodeGate 配额角色: guest / developer / vip
	PolicyName          string   `json:"policy_name"`
	DailyLimit          float64  `json:"daily_limit"`
	WeeklyLimit         float64  `json:"weekly_limit"`
	DailyConsumed       float64  `json:"daily_consumed"`
	WeeklyConsumed      float64  `json:"weekly_consumed"`
	CustomDailyCredits  *float64 `json:"custom_daily_credits,omitempty"`
	CustomWeeklyCredits *float64 `json:"custom_weekly_credits,omitempty"`
}

// HandleAdminListUsers 管理员查询用户及其 CodeGate 配额台账
func HandleAdminListUsers(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	search := strings.TrimSpace(c.Query("search"))

	var users []commonModels.User
	query := db.Model(&commonModels.User{})
	if search != "" {
		query = query.Where("username ILIKE ? OR name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if err := query.Limit(100).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败: " + err.Error()})
		return
	}

	var results []UserQuotaDetailDTO
	for _, u := range users {
		quota, wallet, _ := store.InitOrGetUserQuota(db, u.ID)
		dto := UserQuotaDetailDTO{
			UserID:         u.ID,
			Username:       u.Username,
			Name:           u.Name,
			Email:          u.Email,
			Role:           models.RoleGuest,
			PolicyName:     "未绑定",
			DailyLimit:     50.0,
			WeeklyLimit:    200.0,
			DailyConsumed:  0,
			WeeklyConsumed: 0,
		}
		if quota != nil {
			dto.Role = quota.Role
			dto.CustomDailyCredits = quota.CustomDailyCredits
			dto.CustomWeeklyCredits = quota.CustomWeeklyCredits
			if quota.Policy != nil {
				dto.PolicyName = quota.Policy.Name
				dto.DailyLimit = quota.Policy.DailyCreditsLimit
				dto.WeeklyLimit = quota.Policy.WeeklyCreditsLimit
			}
			if quota.CustomDailyCredits != nil && *quota.CustomDailyCredits > 0 {
				dto.DailyLimit = *quota.CustomDailyCredits
			}
			if quota.CustomWeeklyCredits != nil && *quota.CustomWeeklyCredits > 0 {
				dto.WeeklyLimit = *quota.CustomWeeklyCredits
			}
		}
		if wallet != nil {
			dto.DailyConsumed = wallet.DailyConsumed
			dto.WeeklyConsumed = wallet.WeeklyConsumed
		}
		results = append(results, dto)
	}

	c.JSON(http.StatusOK, gin.H{"data": results, "total": len(results)})
}

// UpdateUserQuotaReq 更新用户配额请求体
type UpdateUserQuotaReq struct {
	Role                string   `json:"role"` // guest / developer / vip
	PolicyID            *uint    `json:"policy_id"`
	CustomDailyCredits  *float64 `json:"custom_daily_credits"`
	CustomWeeklyCredits *float64 `json:"custom_weekly_credits"`
}

// HandleAdminUpdateUserQuota 管理员调整用户配额与角色
func HandleAdminUpdateUserQuota(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	idStr := c.Param("id")
	targetUID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
		return
	}

	var req UpdateUserQuotaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数不合法: " + err.Error()})
		return
	}

	quota, _, err := store.InitOrGetUserQuota(db, uint(targetUID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户配额失败: " + err.Error()})
		return
	}

	// 更新角色
	if req.Role != "" {
		quota.Role = req.Role
	}
	if req.PolicyID != nil {
		quota.PolicyID = req.PolicyID
	}
	quota.CustomDailyCredits = req.CustomDailyCredits

	// 若仅指定日限额未指定周限额，自动按 4 倍联动
	if req.CustomDailyCredits != nil && req.CustomWeeklyCredits == nil {
		w := *req.CustomDailyCredits * 4.0
		quota.CustomWeeklyCredits = &w
	} else {
		quota.CustomWeeklyCredits = req.CustomWeeklyCredits
	}

	if err := db.Save(quota).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存用户配额失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "用户配额更新成功",
		"quota":   quota,
	})
}

// HandleAdminListPolicies 查询配额策略列表
func HandleAdminListPolicies(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var policies []models.QuotaPolicy
	if err := db.Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询策略失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": policies})
}

// HandleAdminSavePolicy 保存或更新配额策略
func HandleAdminSavePolicy(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var policy models.QuotaPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 确保周配额为日配额 4 倍
	if policy.WeeklyCreditsLimit <= 0 && policy.DailyCreditsLimit > 0 {
		policy.WeeklyCreditsLimit = policy.DailyCreditsLimit * 4.0
	}

	if policy.ID == 0 {
		if err := db.Create(&policy).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建策略失败: " + err.Error()})
			return
		}
	} else {
		if err := db.Save(&policy).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新策略失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "策略保存成功", "data": policy})
}

// HandleAdminSaveModel 保存或更新逻辑模型
func HandleAdminSaveModel(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var m models.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	if m.Multiplier <= 0 {
		m.Multiplier = 1.0
	}

	if m.ID == 0 {
		if err := db.Create(&m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建模型失败: " + err.Error()})
			return
		}
	} else {
		if err := db.Save(&m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模型失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "模型保存成功", "data": m})
}

// HandleAdminSaveBackend 保存物理 Backend 实例
func HandleAdminSaveBackend(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var b models.Backend
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	if b.Weight <= 0 {
		b.Weight = 1
	}
	if b.MaxConcurrency <= 0 {
		b.MaxConcurrency = 10
	}
	if len(b.DeclaredProtocols) == 0 {
		protos, _ := json.Marshal([]string{models.ProtocolChat})
		b.DeclaredProtocols = datatypes.JSON(protos)
	}

	if b.ID == 0 {
		if err := db.Create(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建物理后端失败: " + err.Error()})
			return
		}
	} else {
		if err := db.Save(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新物理后端失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "物理实例保存成功", "data": b})
}

// HandleAdminListLogs 审计日志多维检索
func HandleAdminListLogs(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "25"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	query := db.Model(&models.AccessLog{})

	if modelName := c.Query("model"); modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if statusCode := c.Query("statusCode"); statusCode != "" {
		query = query.Where("status_code = ?", statusCode)
	}

	var total int64
	query.Count(&total)

	var logs []models.AccessLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询日志失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}
