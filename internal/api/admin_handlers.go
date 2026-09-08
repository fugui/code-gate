package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code-common/backend/auth"
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
						if r == "admin" || r == "super_admin" || r == "gate_admin" {
							isAdmin = true
							break
						}
					}
				}
			}
		}

		if !isAdmin {
			if rawQuota, exists := c.Get(ContextUserQuota); exists {
				if q, ok := rawQuota.(*models.GateUserQuota); ok && q != nil && q.Role == models.RoleAdmin {
					isAdmin = true
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

	var quotas []models.GateUserQuota
	query := db.Model(&models.GateUserQuota{}).Preload("Policy")
	if search != "" {
		query = query.Where("role ILIKE ? OR CAST(user_id AS TEXT) ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := query.Order("id desc").Limit(100).Find(&quotas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户配额失败: " + err.Error()})
		return
	}

	var results []UserQuotaDetailDTO
	for _, q := range quotas {
		var wallet models.CreditsWallet
		_ = db.Where("user_id = ?", q.UserID).First(&wallet).Error

		policyName := "默认策略"
		dailyLimit := 50.0
		weeklyLimit := 200.0
		if q.Policy != nil {
			policyName = q.Policy.Name
			dailyLimit = q.Policy.DailyCreditsLimit
			weeklyLimit = q.Policy.WeeklyCreditsLimit
		}
		if q.CustomDailyCredits != nil && *q.CustomDailyCredits > 0 {
			dailyLimit = *q.CustomDailyCredits
		}
		if q.CustomWeeklyCredits != nil && *q.CustomWeeklyCredits > 0 {
			weeklyLimit = *q.CustomWeeklyCredits
		}

		dto := UserQuotaDetailDTO{
			UserID:              q.UserID,
			Username:            fmt.Sprintf("用户 #%d", q.UserID),
			Name:                fmt.Sprintf("UID-%d", q.UserID),
			Email:               fmt.Sprintf("user_%d@internal", q.UserID),
			Role:                q.Role,
			PolicyName:          policyName,
			DailyLimit:          dailyLimit,
			WeeklyLimit:         weeklyLimit,
			DailyConsumed:       wallet.DailyConsumed,
			WeeklyConsumed:      wallet.WeeklyConsumed,
			CustomDailyCredits:  q.CustomDailyCredits,
			CustomWeeklyCredits: q.CustomWeeklyCredits,
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

// HandleAdminListBackends 查询所有物理后端节点
func HandleAdminListBackends(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var backends []models.Backend
	if err := db.Order("id ASC").Find(&backends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询后端列表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": backends})
}

// HandleAdminDeleteBackend 删除物理后端
func HandleAdminDeleteBackend(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	if err := db.Delete(&models.Backend{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除后端失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "物理后端已删除"})
}

// HandleAdminDeleteModel 删除逻辑模型
func HandleAdminDeleteModel(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	if err := db.Delete(&models.Model{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模型失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "逻辑模型已删除"})
}

// DashboardSummaryDTO 聚合指标概要卡
type DashboardSummaryDTO struct {
	TotalRequests   int64   `json:"total_requests"`
	TodayRequests   int64   `json:"today_requests"`
	TotalCredits    float64 `json:"total_credits"`
	TodayCredits    float64 `json:"today_credits"`
	ActiveUsers24h  int64   `json:"active_users_24h"`
	TotalBackends   int     `json:"total_backends"`
	HealthyBackends int     `json:"healthy_backends"`
	AvgLatencyMS    int64   `json:"avg_latency_ms"`
}

// HourlyTrendDTO 小时级趋势点
type HourlyTrendDTO struct {
	Hour        string  `json:"hour"`
	Requests    int64   `json:"requests"`
	CostCredits float64 `json:"cost_credits"`
	Errors      int64   `json:"errors"`
}

// TopModelDTO 热门模型排行项
type TopModelDTO struct {
	Model       string  `json:"model"`
	Count       int64   `json:"count"`
	CostCredits float64 `json:"cost_credits"`
	Percentage  float64 `json:"percentage"`
}

// StatusCountsDTO 状态码分布
type StatusCountsDTO struct {
	Status2xx int64 `json:"status_2xx"`
	Status4xx int64 `json:"status_4xx"`
	Status5xx int64 `json:"status_5xx"`
}

// DashboardDataDTO 监控大屏完整数据模型
type DashboardDataDTO struct {
	Summary      DashboardSummaryDTO `json:"summary"`
	HourlyTrends []HourlyTrendDTO    `json:"hourly_trends"`
	TopModels    []TopModelDTO       `json:"top_models"`
	StatusCounts StatusCountsDTO     `json:"status_counts"`
	RecentErrors []models.AccessLog  `json:"recent_errors"`
}

// HandleAdminGetDashboard 管理员监控大屏聚合数据接口
func HandleAdminGetDashboard(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	past24h := now.Add(-24 * time.Hour)

	var data DashboardDataDTO

	// 1. Summary 指标统计
	_ = db.Model(&models.AccessLog{}).Count(&data.Summary.TotalRequests)
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ?", todayStart).Count(&data.Summary.TodayRequests)

	var totalCredits float64
	_ = db.Model(&models.AccessLog{}).Select("COALESCE(SUM(cost_credits), 0)").Scan(&totalCredits)
	data.Summary.TotalCredits = math.Round(totalCredits*100) / 100

	var todayCredits float64
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ?", todayStart).Select("COALESCE(SUM(cost_credits), 0)").Scan(&todayCredits)
	data.Summary.TodayCredits = math.Round(todayCredits*100) / 100

	_ = db.Model(&models.AccessLog{}).Where("created_at >= ?", past24h).Distinct("user_id").Count(&data.Summary.ActiveUsers24h)

	var avgLatency float64
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ?", past24h).Select("COALESCE(AVG(duration_ms), 0)").Scan(&avgLatency)
	data.Summary.AvgLatencyMS = int64(avgLatency)

	var backends []models.Backend
	if err := db.Find(&backends).Error; err == nil {
		data.Summary.TotalBackends = len(backends)
		healthy := 0
		for _, b := range backends {
			if b.IsHealthy {
				healthy++
			}
		}
		data.Summary.HealthyBackends = healthy
	}

	// 2. 过去 24 小时逐小时趋势
	type HourlyRow struct {
		Hr       string  `gorm:"column:hr"`
		ReqCount int64   `gorm:"column:req_count"`
		Credits  float64 `gorm:"column:credits"`
		ErrCount int64   `gorm:"column:err_count"`
	}

	var hourlyRows []HourlyRow
	_ = db.Model(&models.AccessLog{}).
		Select("to_char(created_at, 'YYYY-MM-DD HH24:00') as hr, count(*) as req_count, COALESCE(sum(cost_credits), 0) as credits, count(CASE WHEN status_code >= 400 THEN 1 END) as err_count").
		Where("created_at >= ?", past24h).
		Group("hr").
		Scan(&hourlyRows)

	hourlyMap := make(map[string]HourlyRow, len(hourlyRows))
	for _, r := range hourlyRows {
		hourlyMap[r.Hr] = r
	}

	data.HourlyTrends = make([]HourlyTrendDTO, 0, 24)
	for i := 23; i >= 0; i-- {
		slotTime := now.Add(-time.Duration(i) * time.Hour)
		slotKey := slotTime.Format("2006-01-02 15:00")
		slotDisplay := slotTime.Format("15:00")

		if row, exists := hourlyMap[slotKey]; exists {
			data.HourlyTrends = append(data.HourlyTrends, HourlyTrendDTO{
				Hour:        slotDisplay,
				Requests:    row.ReqCount,
				CostCredits: math.Round(row.Credits*100) / 100,
				Errors:      row.ErrCount,
			})
		} else {
			data.HourlyTrends = append(data.HourlyTrends, HourlyTrendDTO{
				Hour:        slotDisplay,
				Requests:    0,
				CostCredits: 0,
				Errors:      0,
			})
		}
	}

	// 3. 热门模型 TOP 5 (过去 24 小时)
	type TopModelRow struct {
		Model   string  `gorm:"column:model"`
		Count   int64   `gorm:"column:count"`
		Credits float64 `gorm:"column:credits"`
	}
	var topRows []TopModelRow
	_ = db.Model(&models.AccessLog{}).
		Select("model, count(*) as count, COALESCE(sum(cost_credits), 0) as credits").
		Where("created_at >= ?", past24h).
		Group("model").
		Order("count DESC").
		Limit(5).
		Scan(&topRows)

	var past24hTotalRequests int64
	for _, tr := range topRows {
		past24hTotalRequests += tr.Count
	}

	data.TopModels = make([]TopModelDTO, 0, len(topRows))
	for _, tr := range topRows {
		pct := 0.0
		if past24hTotalRequests > 0 {
			pct = math.Round((float64(tr.Count)/float64(past24hTotalRequests))*1000) / 10
		}
		data.TopModels = append(data.TopModels, TopModelDTO{
			Model:       tr.Model,
			Count:       tr.Count,
			CostCredits: math.Round(tr.Credits*100) / 100,
			Percentage:  pct,
		})
	}

	// 4. 状态分布 (最近 24 小时)
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ? AND status_code >= 200 AND status_code < 300", past24h).Count(&data.StatusCounts.Status2xx)
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ? AND status_code >= 400 AND status_code < 500", past24h).Count(&data.StatusCounts.Status4xx)
	_ = db.Model(&models.AccessLog{}).Where("created_at >= ? AND status_code >= 500", past24h).Count(&data.StatusCounts.Status5xx)

	// 5. 最近异常错误日志 (TOP 5)
	_ = db.Model(&models.AccessLog{}).Where("status_code >= 400").Order("id DESC").Limit(5).Find(&data.RecentErrors)

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}


