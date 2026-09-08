package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"code-gate/internal/config"
	"code-gate/internal/models"
	"code-gate/internal/proxy"
	"code-gate/internal/quota"
	"code-gate/internal/routing"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HandleListModels 兼容 OpenAI /v1/models 标准接口
func HandleListModels(c *gin.Context) {
	db := store.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库未就绪"})
		return
	}

	var modelList []models.Model
	if err := db.Where("is_enabled = ?", true).Find(&modelList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询模型列表失败"})
		return
	}

	type ModelItem struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	var data []ModelItem
	for _, m := range modelList {
		data = append(data, ModelItem{
			ID:      m.Name,
			Object:  "model",
			Created: m.CreatedAt.Unix(),
			OwnedBy: "code-gate",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// HandleChatCompletions 核心 OpenAI ChatCompletions 代理端点
func HandleChatCompletions(
	proxyClient *proxy.Client,
	r *routing.Router,
	q *quota.Engine,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		handleProxyRequest(c, proxyClient, r, q, models.ProtocolChat, false)
	}
}

// HandleResponses 原生 Responses 直通代理端点（专为现代 Coding Agent 设计）
func HandleResponses(
	proxyClient *proxy.Client,
	r *routing.Router,
	q *quota.Engine,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		handleProxyRequest(c, proxyClient, r, q, models.ProtocolResponses, true)
	}
}

// handleProxyRequest 统一的协议感知代理核心处理管道
func handleProxyRequest(
	c *gin.Context,
	proxyClient *proxy.Client,
	r *routing.Router,
	q *quota.Engine,
	proto string,
	isResponses bool,
) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "读取请求正文失败", "type": "invalid_request_error"},
		})
		return
	}

	// 轻量解析路由关键字段
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": "解析请求 JSON 失败", "type": "invalid_request_error"},
		})
		return
	}

	db := store.GetDB()
	if db == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": "数据库未连接", "type": "internal_error"},
		})
		return
	}

	// 1. 获取调用者用户 ID 并进行配额预检 (双周期 Credits 限额与 RPM 限流)
	var userID uint
	if uid, exists := c.Get(ContextUserID); exists {
		if u, ok := uid.(uint); ok {
			userID = u
		}
	}

	if userID > 0 && q != nil {
		if _, _, qErr := q.CheckQuota(db, userID, req.Model); qErr != nil {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": qErr.Error(),
					"type":    "quota_exceeded_error",
					"code":    "quota_exceeded",
				},
			})
			return
		}
	}

	// 2. 查询逻辑模型与物理后端
	var targetModel models.Model
	err = db.Preload("Backends").Where("name = ? AND is_enabled = ?", req.Model, true).First(&targetModel).Error
	if err != nil {
		// 尝试默认模型 Fallback 降级
		defaultModelName := config.Get().Defaults.DefaultModel
		if defaultModelName != "" && defaultModelName != req.Model {
			log.Printf("[CodeGate] 请求模型 %s 不可用，正在尝试降级至默认模型 %s", req.Model, defaultModelName)
			if fbErr := db.Preload("Backends").Where("name = ? AND is_enabled = ?", defaultModelName, true).First(&targetModel).Error; fbErr == nil {
				var rawMap map[string]interface{}
				if json.Unmarshal(bodyBytes, &rawMap) == nil {
					rawMap["model"] = defaultModelName
					bodyBytes, _ = json.Marshal(rawMap)
				}
			}
		}
	}

	if targetModel.ID == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "所请求的模型未配置或未启用",
				"type":    "model_not_found",
				"code":    "model_not_found",
			},
		})
		return
	}

	// 3. 协议感知路由选择物理后端 (仅分发至支持该协议的后端)
	selectedBackend, routeErr := r.SelectBackend(&targetModel, proto, routing.StrategyWeightedLeastConn)
	if routeErr != nil {
		if errors.Is(routeErr, routing.ErrNoCompatibleBackend) {
			c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
				"error": gin.H{
					"message": routeErr.Error(),
					"type":    "unsupported_protocol_error",
					"code":    "protocol_not_supported",
				},
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": routeErr.Error(),
				"type":    "service_unavailable",
			},
		})
		return
	}

	// 4. 实例级并发控制（CAS 原子占槽）
	if !selectedBackend.AcquireSlot() {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{
				"message": "所选物理算力实例并发已达上限，请稍后重试",
				"type":    "rate_limit_error",
			},
		})
		return
	}
	defer selectedBackend.ReleaseSlot()

	// 5. 代理转发
	var result *proxy.ProxyResult
	var forwardErr error

	if isResponses {
		result, forwardErr = proxyClient.ForwardResponses(c, selectedBackend, bodyBytes, req.Stream)
	} else {
		result, forwardErr = proxyClient.ForwardChatCompletions(c, selectedBackend, bodyBytes, req.Stream)
	}

	// 6. 异步落盘审计日志与 Credits 扣减
	go recordAuditAndDeductCredits(c.Copy(), targetModel, selectedBackend, proto, result, forwardErr)
}

// recordAuditAndDeductCredits 异步记账与日志落盘
func recordAuditAndDeductCredits(
	c *gin.Context,
	model models.Model,
	backend *models.Backend,
	proto string,
	result *proxy.ProxyResult,
	forwardErr error,
) {
	db := store.GetDB()
	if db == nil || result == nil {
		return
	}

	var userID uint
	if uid, exists := c.Get(ContextUserID); exists {
		if u, ok := uid.(uint); ok {
			userID = u
		}
	}

	var apiKeyID *uint
	if kid, exists := c.Get(ContextAPIKeyID); exists {
		if k, ok := kid.(uint); ok {
			apiKeyID = &k
		}
	}

	credits := proxy.CalculateCredits(result.Usage, model.Multiplier)

	errMsg := ""
	if forwardErr != nil {
		errMsg = forwardErr.Error()
	} else if result.Error != nil {
		errMsg = result.Error.Error()
	}

	// 1. 记录 AccessLog
	accessLog := models.AccessLog{
		UserID:         userID,
		APIKeyID:       apiKeyID,
		ClientIP:       c.ClientIP(),
		UserAgent:      c.Request.UserAgent(),
		Path:           c.Request.URL.Path,
		Protocol:       proto,
		Model:          model.Name,
		InputTokens:    result.Usage.PromptTokens,
		CacheHitTokens: result.Usage.CachedTokens,
		OutputTokens:   result.Usage.CompletionTokens,
		CostCredits:    credits,
		DurationMS:     result.DurationMS,
		TTFTMS:         result.TTFTMS,
		StatusCode:     result.StatusCode,
		ErrorMessage:   errMsg,
		CreatedAt:      time.Now(),
	}
	_ = db.Create(&accessLog)

	// 2. 累加并扣减 Credits
	if userID > 0 && credits > 0 {
		_ = db.Model(&models.CreditsWallet{}).
			Where("user_id = ?", userID).
			Updates(map[string]interface{}{
				"daily_consumed":  gorm.Expr("daily_consumed + ?", credits),
				"weekly_consumed": gorm.Expr("weekly_consumed + ?", credits),
			})
	}
}
