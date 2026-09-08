package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
