package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"code-gate/internal/models"
	"code-gate/internal/store"
	"gorm.io/datatypes"
)

// Prober 后端协议能力自动探针
type Prober struct {
	client   *http.Client
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewProber 创建探针实例
func NewProber(timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Prober{
		client: &http.Client{
			Timeout: timeout,
		},
		stopChan: make(chan struct{}),
	}
}

var (
	globalProber     *Prober
	globalProberLock sync.RWMutex
)

// SetGlobalProber 设置全局 Prober 实例
func SetGlobalProber(p *Prober) {
	globalProberLock.Lock()
	defer globalProberLock.Unlock()
	globalProber = p
}

// GetGlobalProber 获取全局 Prober 实例
func GetGlobalProber() *Prober {
	globalProberLock.RLock()
	defer globalProberLock.RUnlock()
	return globalProber
}

// ProbeBackend 探测单个后端实例的基础健康状态、协议支持能力与网络时延
func (p *Prober) ProbeBackend(ctx context.Context, b *models.Backend) (bool, []string, int64) {
	baseURL := strings.TrimRight(b.BaseURL, "/")

	start := time.Now()

	// 1. 探测 OpenAI Chat 协议支持与基本健康度
	chatURL := baseURL + "/chat/completions"
	if !strings.HasSuffix(baseURL, "/v1") && !strings.Contains(baseURL, "/v1/") {
		chatURL = baseURL + "/v1/chat/completions"
	}

	chatReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return false, []string{}, 0
	}
	chatReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		chatReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	chatResp, err := p.client.Do(chatReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		// 网络不可达或超时
		return false, []string{}, latency
	}
	chatResp.Body.Close()

	// 只要不是 502/503/504 等服务不可用，即视为主机在线
	isHealthy := chatResp.StatusCode != http.StatusBadGateway &&
		chatResp.StatusCode != http.StatusServiceUnavailable &&
		chatResp.StatusCode != http.StatusGatewayTimeout

	supportedProtos := []string{models.ProtocolChat}

	// 2. 探测 Responses 协议支持
	responsesURL := baseURL + "/responses"
	if !strings.HasSuffix(baseURL, "/v1") && !strings.Contains(baseURL, "/v1/") {
		responsesURL = baseURL + "/v1/responses"
	}

	respReq, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesURL, bytes.NewReader([]byte("{}")))
	if err == nil {
		respReq.Header.Set("Content-Type", "application/json")
		if b.APIKey != "" {
			respReq.Header.Set("Authorization", "Bearer "+b.APIKey)
		}
		respRes, err := p.client.Do(respReq)
		if err == nil {
			respRes.Body.Close()
			// 若返回 404 或 405，明确表示不支持 responses 接口；
			// 若返回 200, 400 (Bad Request), 422 (Unprocessable), 401 说明端点存在且激活
			if respRes.StatusCode != http.StatusNotFound && respRes.StatusCode != http.StatusMethodNotAllowed {
				supportedProtos = append(supportedProtos, models.ProtocolResponses)
			}
		}
	}

	return isHealthy, supportedProtos, latency
}

// CheckAll 执行一次全量后端实例探活与协议打标
func (p *Prober) CheckAll(ctx context.Context) {
	db := store.GetDB()
	if db == nil {
		return
	}

	var backends []models.Backend
	if err := db.Find(&backends).Error; err != nil {
		return
	}

	now := time.Now()
	for i := range backends {
		b := &backends[i]
		healthy, protos, latency := p.ProbeBackend(ctx, b)

		protosJSON, _ := json.Marshal(protos)
		consecutiveFailures := 0
		if !healthy {
			consecutiveFailures = b.ConsecutiveFailures + 1
		}

		_ = db.Model(&models.Backend{}).
			Where("id = ?", b.ID).
			Updates(map[string]interface{}{
				"is_healthy":           healthy,
				"detected_protocols":   datatypes.JSON(protosJSON),
				"latency_ms":           latency,
				"consecutive_failures": consecutiveFailures,
				"last_check_at":        now,
			})

		log.Printf("[Prober] 后端实例 [%s] 探测完成: Healthy=%v, Latency=%dms, Failures=%d, Protocols=%v",
			b.Name, healthy, latency, consecutiveFailures, protos)
	}
}

// Start 启动后台定时探测任务
func (p *Prober) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 启动立即执行一次快速探查
		p.CheckAll(context.Background())

		for {
			select {
			case <-ticker.C:
				p.CheckAll(context.Background())
			case <-p.stopChan:
				return
			}
		}
	}()
}

// Stop 停止探针协程
func (p *Prober) Stop() {
	close(p.stopChan)
	p.wg.Wait()
}
