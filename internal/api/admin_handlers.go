package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"code-common/backend/auth"
	"code-gate/internal/models"
	"code-gate/internal/routing"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// 运行时 Server 超时配置快照
var runtimeServerConfig struct {
	ReadTimeout    string
	WriteTimeout   string
	IdleTimeout    string
	MaxHeaderBytes int
}

// SetRuntimeServerConfig 注入运行时 Server 超时配置快照
func SetRuntimeServerConfig(read, write, idle string, maxHeaderBytes int) {
	runtimeServerConfig.ReadTimeout = read
	runtimeServerConfig.WriteTimeout = write
	runtimeServerConfig.IdleTimeout = idle
	runtimeServerConfig.MaxHeaderBytes = maxHeaderBytes
}

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
				if q, ok := rawQuota.(*models.GateUserQuota); ok && q != nil && q.Policy != nil && q.Policy.Name == "admin_policy" {
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

// fetchUserMap 批量反查用户画像信息（姓名、部门、邮箱）
func fetchUserMap(db *gorm.DB, userIDs []uint) map[uint]models.User {
	userMap := make(map[uint]models.User, len(userIDs))
	if len(userIDs) == 0 || db == nil {
		return userMap
	}
	var users []models.User
	if err := db.Preload("Department").Where("id IN ?", userIDs).Find(&users).Error; err == nil {
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	return userMap
}

// =========================================================================
// 1. 用户与配额管理
// =========================================================================

// UserQuotaDetailDTO 用户及其配额消耗视图
type UserQuotaDetailDTO struct {
	UserID              uint     `json:"user_id"`
	Username            string   `json:"username"`
	Name                string   `json:"name"`
	Email               string   `json:"email"`
	Department          string   `json:"department"`
	PolicyID            *uint    `json:"policy_id,omitempty"`
	PolicyName          string   `json:"policy_name"`
	IsCustom            bool     `json:"is_custom"`
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
		query = query.Where("CAST(user_id AS TEXT) ILIKE ?", "%"+search+"%")
	}
	if err := query.Order("id desc").Limit(100).Find(&quotas).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户配额失败: " + err.Error()})
		return
	}

	userIDs := make([]uint, 0, len(quotas))
	for _, q := range quotas {
		userIDs = append(userIDs, q.UserID)
	}
	userMap := fetchUserMap(db, userIDs)

	results := make([]UserQuotaDetailDTO, 0, len(quotas))
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
		isCustom := false
		if q.CustomDailyCredits != nil && *q.CustomDailyCredits > 0 {
			dailyLimit = *q.CustomDailyCredits
			isCustom = true
		}
		if q.CustomWeeklyCredits != nil && *q.CustomWeeklyCredits > 0 {
			weeklyLimit = *q.CustomWeeklyCredits
			isCustom = true
		}

		uName := fmt.Sprintf("UID-%d", q.UserID)
		uUsername := fmt.Sprintf("用户 #%d", q.UserID)
		uEmail := fmt.Sprintf("user_%d@internal", q.UserID)
		uDept := "技术研发中心"

		if u, exists := userMap[q.UserID]; exists {
			if u.Name != "" {
				uName = u.Name
			}
			if u.Username != "" {
				uUsername = u.Username
			}
			if u.Email != "" {
				uEmail = u.Email
			}
			if u.Department != nil && u.Department.Name != "" {
				uDept = u.Department.Name
			}
		}

		dto := UserQuotaDetailDTO{
			UserID:              q.UserID,
			Username:            uUsername,
			Name:                uName,
			Email:               uEmail,
			Department:          uDept,
			PolicyID:            q.PolicyID,
			PolicyName:          policyName,
			IsCustom:            isCustom,
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
	PolicyID            *uint    `json:"policy_id"`
	IsCustom            bool     `json:"is_custom"`
	CustomDailyCredits  *float64 `json:"custom_daily_credits"`
	CustomWeeklyCredits *float64 `json:"custom_weekly_credits"`
}

// HandleAdminUpdateUserQuota 管理员调整用户配额与策略
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

	updates := map[string]interface{}{}
	if req.PolicyID != nil {
		updates["policy_id"] = req.PolicyID
		quota.PolicyID = req.PolicyID
	}

	if req.IsCustom {
		updates["custom_daily_credits"] = req.CustomDailyCredits
		quota.CustomDailyCredits = req.CustomDailyCredits
		// 若仅指定日限额未指定周限额，自动按 4 倍联动
		if req.CustomDailyCredits != nil && req.CustomWeeklyCredits == nil {
			w := *req.CustomDailyCredits * 4.0
			updates["custom_weekly_credits"] = &w
			quota.CustomWeeklyCredits = &w
		} else {
			updates["custom_weekly_credits"] = req.CustomWeeklyCredits
			quota.CustomWeeklyCredits = req.CustomWeeklyCredits
		}
	} else {
		// 非自定义配额时，清空定制限额，完全继承策略配置
		updates["custom_daily_credits"] = nil
		updates["custom_weekly_credits"] = nil
		quota.CustomDailyCredits = nil
		quota.CustomWeeklyCredits = nil
	}

	if err := db.Model(&models.GateUserQuota{}).Where("id = ?", quota.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存用户配额失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "用户配额更新成功",
		"quota":   quota,
	})
}

// =========================================================================
// 2. 配额策略池管理
// =========================================================================

// QuotaPolicyDTO 配额策略返回模型（附带绑定用户统计）
type QuotaPolicyDTO struct {
	models.QuotaPolicy
	UserCount int64 `json:"user_count"`
}

// HandleAdminListPolicies 查询配额策略列表
func HandleAdminListPolicies(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var policies []models.QuotaPolicy
	if err := db.Order("id ASC").Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询策略失败"})
		return
	}

	dtos := make([]QuotaPolicyDTO, 0, len(policies))
	for _, p := range policies {
		var cnt int64
		_ = db.Model(&models.GateUserQuota{}).Where("policy_id = ?", p.ID).Count(&cnt)
		dtos = append(dtos, QuotaPolicyDTO{
			QuotaPolicy: p,
			UserCount:   cnt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": dtos})
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

	// 确保周配额为日配额 4 倍默认联动
	if policy.WeeklyCreditsLimit <= 0 && policy.DailyCreditsLimit > 0 {
		policy.WeeklyCreditsLimit = policy.DailyCreditsLimit * 4.0
	}
	if len(policy.ModelWhitelist) == 0 {
		all, _ := json.Marshal([]string{"*"})
		policy.ModelWhitelist = datatypes.JSON(all)
	}
	if len(policy.TimeRanges) == 0 {
		policy.TimeRanges = datatypes.JSON("[]")
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

// HandleAdminDeletePolicy 删除指定配额策略（安全性约束校验）
func HandleAdminDeletePolicy(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	idStr := c.Param("id")
	policyID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的策略 ID"})
		return
	}

	var boundCount int64
	_ = db.Model(&models.GateUserQuota{}).Where("policy_id = ?", policyID).Count(&boundCount)
	if boundCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("该策略当前仍有 %d 位用户绑定，请先为用户更换策略后再执行删除", boundCount),
		})
		return
	}

	if err := db.Delete(&models.QuotaPolicy{}, "id = ?", policyID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除策略失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配额策略已成功删除"})
}

// =========================================================================
// 3. 模型与后端 1:N 层次治理
// =========================================================================

// ModelDetailDTO 逻辑模型视图（包含其下挂载物理后端及统计）
type ModelDetailDTO struct {
	models.Model
	BackendsCount       int `json:"backends_count"`
	ActiveBackendsCount int `json:"active_backends_count"`
}

// HandleAdminListModels 查询所有逻辑模型及其下属物理后端
func HandleAdminListModels(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var modelsList []models.Model
	if err := db.Preload("Backends").Order("id ASC").Find(&modelsList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询模型列表失败: " + err.Error()})
		return
	}

	dtos := make([]ModelDetailDTO, 0, len(modelsList))
	for _, m := range modelsList {
		activeCount := 0
		for _, b := range m.Backends {
			if b.IsEnabled && b.IsHealthy {
				activeCount++
			}
		}
		dtos = append(dtos, ModelDetailDTO{
			Model:               m,
			BackendsCount:       len(m.Backends),
			ActiveBackendsCount: activeCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": dtos})
}

// SaveModelReq 保存逻辑模型请求体（支持同时挂载首个后端）
type SaveModelReq struct {
	ID             uint           `json:"id"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Multiplier     float64        `json:"multiplier"`
	DefaultModel   string         `json:"default_model"`
	ModelParams    datatypes.JSON `json:"model_params"`
	IsEnabled      *bool          `json:"is_enabled"`
	InitialBackend *struct {
		BaseURL        string `json:"base_url"`
		APIKey         string `json:"api_key"`
		Weight         int    `json:"weight"`
		MaxConcurrency int32  `json:"max_concurrency"`
	} `json:"initial_backend,omitempty"`
}

// HandleAdminSaveModel 保存或更新逻辑模型
func HandleAdminSaveModel(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var req SaveModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误: " + err.Error()})
		return
	}

	if req.Multiplier <= 0 {
		req.Multiplier = 1.0
	}
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	if req.ID == 0 {
		m := models.Model{
			Name:         strings.TrimSpace(req.Name),
			Description:  strings.TrimSpace(req.Description),
			Multiplier:   req.Multiplier,
			DefaultModel: strings.TrimSpace(req.DefaultModel),
			ModelParams:  req.ModelParams,
			IsEnabled:    isEnabled,
		}
		if m.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "模型标识 Name 不能为空"})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			if req.InitialBackend != nil && strings.TrimSpace(req.InitialBackend.BaseURL) != "" {
				weight := req.InitialBackend.Weight
				if weight <= 0 {
					weight = 10
				}
				maxC := req.InitialBackend.MaxConcurrency
				if maxC <= 0 {
					maxC = 50
				}
				protos, _ := json.Marshal([]string{models.ProtocolChat})
				b := models.Backend{
					ModelID:           m.ID,
					Name:              fmt.Sprintf("%s-node-1", m.Name),
					BaseURL:           strings.TrimSpace(req.InitialBackend.BaseURL),
					APIKey:            strings.TrimSpace(req.InitialBackend.APIKey),
					Weight:            weight,
					MaxConcurrency:    maxC,
					DeclaredProtocols: datatypes.JSON(protos),
					DetectedProtocols: datatypes.JSON(protos),
					IsHealthy:         true,
					IsEnabled:         true,
				}
				if err := tx.Create(&b).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建模型失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "模型创建成功", "data": m})
	} else {
		var m models.Model
		if err := db.First(&m, req.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "模型不存在"})
			return
		}

		newName := strings.TrimSpace(req.Name)
		if newName != "" && newName != m.Name {
			var count int64
			db.Model(&models.Model{}).Where("name = ? AND id != ?", newName, m.ID).Count(&count)
			if count > 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("模型标识 %s 已被其他模型占用", newName)})
				return
			}
			oldName := m.Name
			m.Name = newName
			// 级联更新其他模型引用的保底降级模型名称
			_ = db.Model(&models.Model{}).Where("default_model = ?", oldName).Update("default_model", newName).Error
			// 级联更新配额策略引用的默认模型名称
			_ = db.Model(&models.QuotaPolicy{}).Where("default_model = ?", oldName).Update("default_model", newName).Error
		}

		m.Description = strings.TrimSpace(req.Description)
		if req.Multiplier > 0 {
			m.Multiplier = req.Multiplier
		}
		m.DefaultModel = strings.TrimSpace(req.DefaultModel)
		if len(req.ModelParams) > 0 {
			m.ModelParams = req.ModelParams
		}
		if req.IsEnabled != nil {
			m.IsEnabled = *req.IsEnabled
		}

		if err := db.Save(&m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模型失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "模型更新成功", "data": m})
	}
}

// HandleAdminToggleModel 切换逻辑模型启用/禁用状态
func HandleAdminToggleModel(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	var m models.Model
	if err := db.First(&m, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模型不存在"})
		return
	}

	m.IsEnabled = !m.IsEnabled
	if err := db.Save(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新模型状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "状态已更新", "is_enabled": m.IsEnabled})
}

// HandleAdminDeleteModel 删除逻辑模型及其关联物理后端
func HandleAdminDeleteModel(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.Backend{}, "model_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.Model{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除模型失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "逻辑模型及关联后端已安全删除"})
}

// ImportModelsReq 网关批量导入请求体
type ImportModelsReq struct {
	Prefix  string `json:"prefix" binding:"required"`
	BaseURL string `json:"base_url" binding:"required"`
	APIKey  string `json:"api_key"`
}

// HandleAdminImportModels 上游网关批量自动导入模型
func HandleAdminImportModels(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var req ImportModelsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要参数: " + err.Error()})
		return
	}

	baseURL := strings.TrimRight(req.BaseURL, "/")
	reqURL := baseURL + "/models"
	if !strings.HasSuffix(baseURL, "/v1") {
		reqURL = baseURL + "/v1/models"
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "构建上游请求失败: " + err.Error()})
		return
	}
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(req.APIKey))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("连接上游接口失败 (%s): %v", reqURL, err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("上游返回错误状态码 %d", resp.StatusCode)})
		return
	}

	var listResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析上游模型列表失败: " + err.Error()})
		return
	}

	if len(listResp.Data) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "上游未返回可用模型列表", "imported_count": 0})
		return
	}

	importedCount := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, item := range listResp.Data {
			modelName := strings.TrimSpace(item.ID)
			if modelName == "" {
				continue
			}

			// 查找或创建 Model
			var m models.Model
			if err := tx.Where("name = ?", modelName).First(&m).Error; err != nil {
				m = models.Model{
					Name:        modelName,
					Description: fmt.Sprintf("由网关批量导入 (前缀: %s)", req.Prefix),
					Multiplier:  1.0,
					IsEnabled:   true,
				}
				if err := tx.Create(&m).Error; err != nil {
					return err
				}
			}

			// 检查是否已存在同 BaseURL 的 Backend
			var existing models.Backend
			if err := tx.Where("model_id = ? AND base_url = ?", m.ID, baseURL).First(&existing).Error; err != nil {
				protos, _ := json.Marshal([]string{models.ProtocolChat})
				backendName := fmt.Sprintf("%s-%s-1", req.Prefix, modelName)
				if len(backendName) > 120 {
					backendName = backendName[:120]
				}
				newBackend := models.Backend{
					ModelID:           m.ID,
					Name:              backendName,
					BaseURL:           baseURL,
					APIKey:            req.APIKey,
					Weight:            10,
					MaxConcurrency:    50,
					DeclaredProtocols: datatypes.JSON(protos),
					DetectedProtocols: datatypes.JSON(protos),
					IsHealthy:         true,
					IsEnabled:         true,
				}
				if err := tx.Create(&newBackend).Error; err != nil {
					return err
				}
			}
			importedCount++
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "批量入库失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("成功从上游导入/同步 %d 个逻辑模型与实例", importedCount),
		"imported_count": importedCount,
	})
}

// HandleAdminListBackends 查询所有物理后端节点（带所属模型名称）
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

// HandleAdminSaveBackend 保存或更新物理 Backend 实例
func HandleAdminSaveBackend(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var b models.Backend
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误: " + err.Error()})
		return
	}
	if b.Weight <= 0 {
		b.Weight = 10
	}
	if b.MaxConcurrency <= 0 {
		b.MaxConcurrency = 50
	}
	if len(b.DeclaredProtocols) == 0 {
		protos, _ := json.Marshal([]string{models.ProtocolChat})
		b.DeclaredProtocols = datatypes.JSON(protos)
	}

	if b.ID == 0 {
		b.IsHealthy = true
		b.IsEnabled = true
		if err := db.Create(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建物理后端失败: " + err.Error()})
			return
		}
	} else {
		var old models.Backend
		if err := db.First(&old, b.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "后端实例不存在"})
			return
		}
		// 若未提供 APIKey 则保留旧密码
		if b.APIKey == "" {
			b.APIKey = old.APIKey
		}
		if err := db.Save(&b).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "更新物理后端失败: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "物理实例保存成功", "data": b})
}

// HandleAdminToggleBackend 切换物理后端启用状态
func HandleAdminToggleBackend(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	id := c.Param("id")
	var b models.Backend
	if err := db.First(&b, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "物理后端不存在"})
		return
	}

	b.IsEnabled = !b.IsEnabled
	if err := db.Save(&b).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新后端状态失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "实例状态已更新", "is_enabled": b.IsEnabled})
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

	c.JSON(http.StatusOK, gin.H{"message": "物理后端已成功删除"})
}

// =========================================================================
// 4. 后端全景健康与并发水位监控
// =========================================================================

// BackendHealthDTO 实例健康度与并发水位视图
type BackendHealthDTO struct {
	ID                  uint           `json:"id"`
	ModelID             uint           `json:"model_id"`
	ModelName           string         `json:"model_name"`
	Name                string         `json:"name"`
	BaseURL             string         `json:"base_url"`
	Weight              int            `json:"weight"`
	MaxConcurrency      int32          `json:"max_concurrency"`
	ActiveConnections   int32          `json:"active_connections"`
	UtilizationRatio    float64        `json:"utilization_ratio"` // 水位比例 0.0 ~ 1.0
	IsHealthy           bool           `json:"is_healthy"`
	IsEnabled           bool           `json:"is_enabled"`
	LatencyMS           int64          `json:"latency_ms"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
	LastCheckAt         *time.Time     `json:"last_check_at,omitempty"`
	DeclaredProtocols   datatypes.JSON `json:"declared_protocols"`
	DetectedProtocols   datatypes.JSON `json:"detected_protocols"`
}

// HealthSummaryDTO 健康总体指标
type HealthSummaryDTO struct {
	TotalBackends   int   `json:"total_backends"`
	HealthyBackends int   `json:"healthy_backends"`
	FaultBackends   int   `json:"fault_backends"`
	AvgLatencyMS    int64 `json:"avg_latency_ms"`
}

// HealthMatrixDTO 健康全景大盘
type HealthMatrixDTO struct {
	Summary  HealthSummaryDTO   `json:"summary"`
	Backends []BackendHealthDTO `json:"backends"`
}

// HandleAdminGetHealth 获取所有后端实例的实时健康与并发状态大盘
func HandleAdminGetHealth(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var backends []models.Backend
	if err := db.Order("id ASC").Find(&backends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询后端健康状态失败: " + err.Error()})
		return
	}

	var modelsList []models.Model
	_ = db.Find(&modelsList)
	modelNameMap := make(map[uint]string, len(modelsList))
	for _, m := range modelsList {
		modelNameMap[m.ID] = m.Name
	}

	var matrix HealthMatrixDTO
	matrix.Summary.TotalBackends = len(backends)

	var totalLatency int64
	latencyCount := 0

	matrix.Backends = make([]BackendHealthDTO, 0, len(backends))
	for _, b := range backends {
		if b.IsHealthy && b.IsEnabled {
			matrix.Summary.HealthyBackends++
		} else {
			matrix.Summary.FaultBackends++
		}

		if b.LatencyMS > 0 {
			totalLatency += b.LatencyMS
			latencyCount++
		}

		active := atomic.LoadInt32(&b.ActiveConnections)
		maxC := b.MaxConcurrency
		if maxC <= 0 {
			maxC = 10
		}
		ratio := math.Round((float64(active)/float64(maxC))*1000) / 1000

		mName := modelNameMap[b.ModelID]
		if mName == "" {
			mName = fmt.Sprintf("Model #%d", b.ModelID)
		}

		dto := BackendHealthDTO{
			ID:                  b.ID,
			ModelID:             b.ModelID,
			ModelName:           mName,
			Name:                b.Name,
			BaseURL:             b.BaseURL,
			Weight:              b.Weight,
			MaxConcurrency:      b.MaxConcurrency,
			ActiveConnections:   active,
			UtilizationRatio:    ratio,
			IsHealthy:           b.IsHealthy,
			IsEnabled:           b.IsEnabled,
			LatencyMS:           b.LatencyMS,
			ConsecutiveFailures: b.ConsecutiveFailures,
			LastCheckAt:         b.LastCheckAt,
			DeclaredProtocols:   b.DeclaredProtocols,
			DetectedProtocols:   b.DetectedProtocols,
		}
		matrix.Backends = append(matrix.Backends, dto)
	}

	if latencyCount > 0 {
		matrix.Summary.AvgLatencyMS = totalLatency / int64(latencyCount)
	}

	c.JSON(http.StatusOK, gin.H{"data": matrix})
}

// HandleAdminTriggerProbe 手动触发一次全量后端探活
func HandleAdminTriggerProbe(c *gin.Context) {
	prober := routing.GetGlobalProber()
	if prober != nil {
		prober.CheckAll(c.Request.Context())
	}
	c.JSON(http.StatusOK, gin.H{"message": "全量后端实例探活与协议打标已完成"})
}

// =========================================================================
// 5. 系统配置与动态客户端安全过滤
// =========================================================================

// SystemConfigDTO 系统运行时配置视图
type SystemConfigDTO struct {
	BlockedUserAgents []string `json:"blocked_user_agents"`
	ReadTimeout       string   `json:"read_timeout"`
	WriteTimeout      string   `json:"write_timeout"`
	IdleTimeout       string   `json:"idle_timeout"`
	MaxHeaderBytes    int      `json:"max_header_bytes"`
}

// HandleAdminGetSystemConfig 获取当前系统运行时配置快照
func HandleAdminGetSystemConfig(c *gin.Context) {
	db := store.GetDB()
	filter := GetGlobalClientFilter(nil)
	var blocked []string
	if filter != nil {
		blocked = filter.GetBlockedUAs()
	}
	if len(blocked) == 0 && db != nil {
		blocked = store.GetBlockedUserAgents(db, []string{"sqlmap", "nikto", "acunetix", "havij", "masscan"})
	}

	read := runtimeServerConfig.ReadTimeout
	if read == "" {
		read = "30m"
	}
	write := runtimeServerConfig.WriteTimeout
	if write == "" {
		write = "30m"
	}
	idle := runtimeServerConfig.IdleTimeout
	if idle == "" {
		idle = "120s"
	}
	maxHeader := runtimeServerConfig.MaxHeaderBytes
	if maxHeader <= 0 {
		maxHeader = 1048576
	}

	data := SystemConfigDTO{
		BlockedUserAgents: blocked,
		ReadTimeout:       read,
		WriteTimeout:      write,
		IdleTimeout:       idle,
		MaxHeaderBytes:    maxHeader,
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// UpdateSystemConfigReq 更新系统配置请求体
type UpdateSystemConfigReq struct {
	BlockedUserAgents []string `json:"blocked_user_agents"`
}

// HandleAdminUpdateSystemConfig 更新系统配置并触发内存热重载
func HandleAdminUpdateSystemConfig(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	var req UpdateSystemConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 1. 持久化至数据库
	if err := store.SaveBlockedUserAgents(db, req.BlockedUserAgents); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	// 2. 动态热重载全局 ClientFilter 中间件
	filter := GetGlobalClientFilter(nil)
	if filter != nil {
		filter.SetBlockedUAs(req.BlockedUserAgents)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "客户端安全过滤规则已热更新，即刻生效",
		"data":    req.BlockedUserAgents,
	})
}

// =========================================================================
// 6. 7天 TOP 算力消费者逐日交叉矩阵大账
// =========================================================================

// DailyConsumerMetric 单日用户消耗单元格
type DailyConsumerMetric struct {
	Date         string  `json:"date"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostCredits  float64 `json:"cost_credits"`
}

// UserTopConsumerRow 用户算力透视行
type UserTopConsumerRow struct {
	UserID       uint                           `json:"user_id"`
	Name         string                         `json:"name"`
	Username     string                         `json:"username"`
	Email        string                         `json:"email"`
	Department   string                         `json:"department"`
	Daily        map[string]DailyConsumerMetric `json:"daily"` // 按 "YYYY-MM-DD" 索引
	TotalReq     int64                          `json:"total_req"`
	TotalTokens  int64                          `json:"total_tokens"`
	TotalCredits float64                        `json:"total_credits"`
}

// GrandTotalRow 全局逐日汇总行
type GrandTotalRow struct {
	Daily        map[string]DailyConsumerMetric `json:"daily"`
	TotalReq     int64                          `json:"total_req"`
	TotalTokens  int64                          `json:"total_tokens"`
	TotalCredits float64                        `json:"total_credits"`
}

// TopConsumersMatrixDTO 7天算力透视全景响应
type TopConsumersMatrixDTO struct {
	Dates      []string             `json:"dates"` // 过去的 7 天列表 ["2026-09-03", ..., "2026-09-09"]
	Users      []UserTopConsumerRow `json:"users"`
	GrandTotal GrandTotalRow        `json:"grand_total"`
}

// HandleAdminGetTopConsumers 获取最近 7 天 TOP 算力消费者逐日交叉矩阵
func HandleAdminGetTopConsumers(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未连接"})
		return
	}

	now := time.Now()
	// 生成 7 天日期列表（按先后顺序，D-6 到 今天）
	dates := make([]string, 0, 7)
	dateMap := make(map[string]bool, 7)
	for i := 6; i >= 0; i-- {
		dStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		dates = append(dates, dStr)
		dateMap[dStr] = true
	}

	startDate := now.AddDate(0, 0, -6)
	startDayTime := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, now.Location())

	type DailyAggRow struct {
		UserID      uint    `gorm:"column:user_id"`
		DayStr      string  `gorm:"column:day_str"`
		ReqCount    int64   `gorm:"column:req_count"`
		InTokens    int64   `gorm:"column:in_tokens"`
		OutTokens   int64   `gorm:"column:out_tokens"`
		CostCredits float64 `gorm:"column:cost_credits"`
	}

	var aggRows []DailyAggRow
	err := db.Model(&models.AccessLog{}).
		Select("user_id, to_char(created_at, 'YYYY-MM-DD') as day_str, count(*) as req_count, COALESCE(sum(input_tokens), 0) as in_tokens, COALESCE(sum(output_tokens), 0) as out_tokens, COALESCE(sum(cost_credits), 0) as cost_credits").
		Where("created_at >= ?", startDayTime).
		Group("user_id, day_str").
		Scan(&aggRows).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "聚合日志失败: " + err.Error()})
		return
	}

	// 收集涉及的用户 ID 并反查真实画像
	userAggMap := make(map[uint]map[string]DailyConsumerMetric)
	userTotals := make(map[uint]*struct {
		req     int64
		tokens  int64
		credits float64
	})

	for _, row := range aggRows {
		if _, ok := userAggMap[row.UserID]; !ok {
			userAggMap[row.UserID] = make(map[string]DailyConsumerMetric)
			userTotals[row.UserID] = &struct {
				req     int64
				tokens  int64
				credits float64
			}{}
		}
		userAggMap[row.UserID][row.DayStr] = DailyConsumerMetric{
			Date:         row.DayStr,
			Requests:     row.ReqCount,
			InputTokens:  row.InTokens,
			OutputTokens: row.OutTokens,
			CostCredits:  math.Round(row.CostCredits*100) / 100,
		}
		userTotals[row.UserID].req += row.ReqCount
		userTotals[row.UserID].tokens += row.InTokens + row.OutTokens
		userTotals[row.UserID].credits += row.CostCredits
	}

	userIDs := make([]uint, 0, len(userAggMap))
	for uid := range userAggMap {
		userIDs = append(userIDs, uid)
	}
	userMap := fetchUserMap(db, userIDs)

	var matrix TopConsumersMatrixDTO
	matrix.Dates = dates
	matrix.GrandTotal.Daily = make(map[string]DailyConsumerMetric)
	for _, d := range dates {
		matrix.GrandTotal.Daily[d] = DailyConsumerMetric{Date: d}
	}

	matrix.Users = make([]UserTopConsumerRow, 0, len(userIDs))
	for _, uid := range userIDs {
		uName := fmt.Sprintf("UID-%d", uid)
		uUsername := fmt.Sprintf("用户 #%d", uid)
		uEmail := fmt.Sprintf("user_%d@internal", uid)
		uDept := "技术研发中心"

		if u, exists := userMap[uid]; exists {
			if u.Name != "" {
				uName = u.Name
			}
			if u.Username != "" {
				uUsername = u.Username
			}
			if u.Email != "" {
				uEmail = u.Email
			}
			if u.Department != nil && u.Department.Name != "" {
				uDept = u.Department.Name
			}
		}

		userDaily := make(map[string]DailyConsumerMetric, 7)
		for _, d := range dates {
			if m, ok := userAggMap[uid][d]; ok {
				userDaily[d] = m
				gt := matrix.GrandTotal.Daily[d]
				gt.Requests += m.Requests
				gt.InputTokens += m.InputTokens
				gt.OutputTokens += m.OutputTokens
				gt.CostCredits += m.CostCredits
				matrix.GrandTotal.Daily[d] = gt
			} else {
				userDaily[d] = DailyConsumerMetric{Date: d}
			}
		}

		tot := userTotals[uid]
		totCredits := math.Round(tot.credits*100) / 100
		matrix.Users = append(matrix.Users, UserTopConsumerRow{
			UserID:       uid,
			Name:         uName,
			Username:     uUsername,
			Email:        uEmail,
			Department:   uDept,
			Daily:        userDaily,
			TotalReq:     tot.req,
			TotalTokens:  tot.tokens,
			TotalCredits: totCredits,
		})

		matrix.GrandTotal.TotalReq += tot.req
		matrix.GrandTotal.TotalTokens += tot.tokens
		matrix.GrandTotal.TotalCredits += tot.credits
	}

	// 统一四舍五入
	matrix.GrandTotal.TotalCredits = math.Round(matrix.GrandTotal.TotalCredits*100) / 100
	for d, gt := range matrix.GrandTotal.Daily {
		gt.CostCredits = math.Round(gt.CostCredits*100) / 100
		matrix.GrandTotal.Daily[d] = gt
	}

	// 按总消耗 Credits 降序排序
	for i := 0; i < len(matrix.Users); i++ {
		for j := i + 1; j < len(matrix.Users); j++ {
			if matrix.Users[i].TotalCredits < matrix.Users[j].TotalCredits {
				matrix.Users[i], matrix.Users[j] = matrix.Users[j], matrix.Users[i]
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": matrix})
}

// =========================================================================
// 7. 审计日志与监控大屏
// =========================================================================

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
			if b.IsHealthy && b.IsEnabled {
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
