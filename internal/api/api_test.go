package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"code-common/backend/auth"
	"code-gate/internal/config"
	"code-gate/internal/models"
	"code-gate/internal/proxy"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
)

func TestHealthAndModelsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 使用 config.yaml.example 初始化测试环境
	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过集成测试: %v", err)
		return
	}

	// 初始化一个测试模型
	testModel := models.Model{
		Name:        "test-deepseek",
		Multiplier:  1.0,
		Description: "测试模型",
		IsEnabled:   true,
	}
	_ = db.Where("name = ?", testModel.Name).FirstOrCreate(&testModel).Error

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	v1 := r.Group("/v1")
	v1.GET("/models", HandleListModels)

	// 1. 测试 /health
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health 状态码错误: 期望 200, 获得 %d", w.Code)
	}

	// 2. 测试 /v1/models
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/v1/models", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("GET /v1/models 状态码错误: 期望 200, 获得 %d", w2.Code)
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析 /v1/models 响应失败: %v", err)
	}
	if resp.Object != "list" || len(resp.Data) == 0 {
		t.Errorf("/v1/models 响应格式不正确: %+v", resp)
	}
}

func TestMockProxyForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 1. 模拟上游 LLM 服务 (Mock Upstream)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// 发送两条 chunk，第二条携带 usage
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		time.Sleep(10 * time.Millisecond)

		_, _ = w.Write([]byte("data: {\"id\":\"2\",\"choices\":[{\"delta\":{\"content\":\" World\"}}],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":500,\"total_tokens\":1500,\"prompt_tokens_details\":{\"cached_tokens\":800}}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	// 2. 初始化代理客户端
	proxyClient := proxy.NewClient(5 * time.Second)

	backend := &models.Backend{
		Name:           "mock-backend",
		BaseURL:        mockServer.URL,
		MaxConcurrency: 5,
		IsHealthy:      true,
	}

	r := gin.New()
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		body := []byte(`{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
		res, err := proxyClient.ForwardChatCompletions(c, backend, body, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 校验提取到的 Usage
		if res.Usage.PromptTokens != 1000 || res.Usage.CachedTokens != 800 || res.Usage.CompletionTokens != 500 {
			t.Errorf("提取到的 Usage 不正确: %+v", res.Usage)
		}

		// 校验计算的 Credits: (200*1 + 800*0.1 + 500*5)/1000 = 2.78
		credits := proxy.CalculateCredits(res.Usage, 1.0)
		if credits != 2.78 {
			t.Errorf("计算得到的 Credits 不正确: 期望 2.78, 获得 %v", credits)
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Mock 代理请求失败，HTTP 状态码: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestResponsesProtocolDirectForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 模拟支持 responses 协议的后端
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" && r.URL.Path != "/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp-123","output":"Agent output"}`))
	}))
	defer mockServer.Close()

	proxyClient := proxy.NewClient(5 * time.Second)
	backend := &models.Backend{
		Name:           "mock-responses-backend",
		BaseURL:        mockServer.URL,
		MaxConcurrency: 5,
		IsHealthy:      true,
	}

	r := gin.New()
	r.POST("/v1/responses", func(c *gin.Context) {
		body := []byte(`{"model":"codex-model","input":"Write code"}`)
		res, err := proxyClient.ForwardResponses(c, backend, body, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if res.StatusCode != http.StatusOK {
			t.Errorf("期望返回 200, 获得 %d", res.StatusCode)
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Responses 直通请求失败: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestUnifiedAuthMiddlewareWithSharedJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sharedSecret := "ABCDEFGHIJKLMNOPQRSTVUWXYZ0987654321"
	wrongSecret := "WrongSecretKey123456789"

	// 1. 初始化数据库
	cfg, err := config.Load("../../config.yaml")
	if err != nil {
		cfg, _ = config.Load("../../config.yaml.example")
	}
	if cfg != nil {
		_, _ = store.InitDB(cfg)
	}

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(UnifiedAuthMiddleware(func() string {
		return sharedSecret
	}))
	v1.GET("/user/profile", HandleGetUserProfile)

	// 正确的统一 JWT
	validToken, err := auth.GenerateToken(999, "testuser", "test@example.com", "Test User", false, []string{"user"}, sharedSecret, 1*time.Hour)
	if err != nil {
		t.Fatalf("生成 validToken 失败: %v", err)
	}

	// 错误的 JWT
	invalidToken, _ := auth.GenerateToken(999, "testuser", "test@example.com", "Test User", false, []string{"user"}, wrongSecret, 1*time.Hour)

	// 测试用错误的 JWT 请求 -> 401
	wInvalid := httptest.NewRecorder()
	reqInvalid, _ := http.NewRequest(http.MethodGet, "/v1/user/profile", nil)
	reqInvalid.Header.Set("Authorization", "Bearer "+invalidToken)
	r.ServeHTTP(wInvalid, reqInvalid)

	if wInvalid.Code != http.StatusUnauthorized {
		t.Errorf("使用错误密钥的 JWT 期望返回 401, 实际获得: %d", wInvalid.Code)
	}

	// 测试用统一共享 JWT 请求 -> 200
	wValid := httptest.NewRecorder()
	reqValid, _ := http.NewRequest(http.MethodGet, "/v1/user/profile", nil)
	reqValid.Header.Set("Authorization", "Bearer "+validToken)
	r.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Errorf("使用统一共享密钥的 JWT 期望返回 200, 实际获得: %d, body: %s", wValid.Code, wValid.Body.String())
	}
}

func TestUserAPIKeyLifecycleWithRawKeyAndNeverExpire(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接数据库，跳过集成测试: %v", err)
		return
	}

	testUserID := uint(8888)
	// 清理旧测试数据
	db.Where("user_id = ?", testUserID).Delete(&models.APIKey{})

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(func(c *gin.Context) {
		c.Set(ContextUserID, testUserID)
		c.Next()
	})
	v1.POST("/user/keys", HandleCreateUserKey)
	v1.GET("/user/keys", HandleListUserKeys)

	// 1. 创建 API Key，未指定 expires_in（即 0，默认永不过期）
	bodyData := []byte(`{"name":"测试永久密钥","expires_in":0}`)
	wCreate := httptest.NewRecorder()
	reqCreate, _ := http.NewRequest(http.MethodPost, "/v1/user/keys", bytes.NewBuffer(bodyData))
	reqCreate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCreate, reqCreate)

	if wCreate.Code != http.StatusOK {
		t.Fatalf("创建 API Key 期望 200, 实际获得: %d, body: %s", wCreate.Code, wCreate.Body.String())
	}

	var createResp struct {
		Data struct {
			ID        uint    `json:"id"`
			Name      string  `json:"name"`
			RawKey    string  `json:"raw_key"`
			ExpiresAt *string `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if createResp.Data.RawKey == "" || !strings.HasPrefix(createResp.Data.RawKey, "sk-gate-") {
		t.Errorf("RawKey 格式不正确: %s", createResp.Data.RawKey)
	}
	if createResp.Data.ExpiresAt != nil {
		t.Errorf("ExpiresAt 期望为 null (永不过期), 实际为: %v", *createResp.Data.ExpiresAt)
	}

	// 2. 查库验证 RawKey 是否已持久化
	var savedKey models.APIKey
	if err := db.First(&savedKey, createResp.Data.ID).Error; err != nil {
		t.Fatalf("查询已保存的 APIKey 失败: %v", err)
	}
	if savedKey.RawKey != createResp.Data.RawKey {
		t.Errorf("数据库持久化 RawKey 期望 %s, 实际 %s", createResp.Data.RawKey, savedKey.RawKey)
	}
	if savedKey.ExpiresAt != nil {
		t.Errorf("数据库持久化 ExpiresAt 期望 nil, 实际 %v", savedKey.ExpiresAt)
	}

	// 3. 调用列表接口，验证是否返回 raw_key 且支持查看与复制
	wList := httptest.NewRecorder()
	reqList, _ := http.NewRequest(http.MethodGet, "/v1/user/keys", nil)
	r.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("获取 API Key 列表期望 200, 实际获得: %d", wList.Code)
	}

	var listResp struct {
		Data []models.APIKey `json:"data"`
	}
	if err := json.Unmarshal(wList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("解析列表响应失败: %v", err)
	}
	if len(listResp.Data) == 0 {
		t.Fatalf("列表期望返回至少 1 条记录")
	}
	found := false
	for _, k := range listResp.Data {
		if k.ID == createResp.Data.ID {
			found = true
			if k.RawKey != createResp.Data.RawKey {
				t.Errorf("列表返回 RawKey 期望 %s, 实际 %s", createResp.Data.RawKey, k.RawKey)
			}
			if k.ExpiresAt != nil {
				t.Errorf("列表返回 ExpiresAt 期望 nil, 实际 %v", k.ExpiresAt)
			}
		}
	}
	if !found {
		t.Errorf("列表中未找到新建的 API Key")
	}
}
