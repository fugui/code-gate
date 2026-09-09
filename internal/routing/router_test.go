package routing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code-gate/internal/models"
	"gorm.io/datatypes"
)

func TestProtocolAwareRouting(t *testing.T) {
	chatOnlyJSON, _ := json.Marshal([]string{models.ProtocolChat})
	bothProtosJSON, _ := json.Marshal([]string{models.ProtocolChat, models.ProtocolResponses})

	backendA := models.Backend{
		ID:                1,
		Name:              "backend-chat-only",
		BaseURL:           "http://backend-a",
		Weight:            1,
		DetectedProtocols: datatypes.JSON(chatOnlyJSON),
		IsHealthy:         true,
	}

	backendB := models.Backend{
		ID:                2,
		Name:              "backend-both",
		BaseURL:           "http://backend-b",
		Weight:            1,
		DetectedProtocols: datatypes.JSON(bothProtosJSON),
		IsHealthy:         true,
	}

	testModel := &models.Model{
		ID:       10,
		Name:     "mixed-model",
		Backends: []models.Backend{backendA, backendB},
	}

	r := NewRouter()

	// 1. 请求 responses 协议：必须只能命中 backendB
	for i := 0; i < 5; i++ {
		selected, err := r.SelectBackend(testModel, models.ProtocolResponses, "", StrategyWeightedLeastConn)
		if err != nil {
			t.Fatalf("选择 responses 后端失败: %v", err)
		}
		if selected.ID != backendB.ID {
			t.Errorf("协议感知路由错误: 期望调度到 backendB (ID=2), 实际得到 ID=%d (%s)", selected.ID, selected.Name)
		}
	}

	// 2. 测试纯 Chat 模型请求 Responses 协议：应返回协议不支持错误
	chatOnlyModel := &models.Model{
		ID:       20,
		Name:     "chat-only-model",
		Backends: []models.Backend{backendA},
	}
	_, err := r.SelectBackend(chatOnlyModel, models.ProtocolResponses, "", StrategyWeightedLeastConn)
	if err == nil {
		t.Fatalf("对不支持 responses 的模型选路应当报错，但得到了 nil")
	}
}

func TestHRWAffinityAndSpillover(t *testing.T) {
	b1 := &models.Backend{ID: 101, Name: "b1", Weight: 1, MaxConcurrency: 2, ActiveConnections: 0}
	b2 := &models.Backend{ID: 102, Name: "b2", Weight: 1, MaxConcurrency: 2, ActiveConnections: 0}
	b3 := &models.Backend{ID: 103, Name: "b3", Weight: 1, MaxConcurrency: 2, ActiveConnections: 0}
	backends := []*models.Backend{b1, b2, b3}

	sessionKey := "chat-session-uuid-12345"

	// 1. 验证会话粘性：相同的 SessionID 在后端稳定时应始终命中同一个节点
	initialBackend := SelectByHRW(sessionKey, backends)
	if initialBackend == nil {
		t.Fatalf("SelectByHRW 返回 nil")
	}

	for i := 0; i < 10; i++ {
		b := SelectByHRW(sessionKey, backends)
		if b.ID != initialBackend.ID {
			t.Errorf("HRW 会话粘性失效: 期望始终命中 %s, 但第 %d 次命中了 %s", initialBackend.Name, i, b.Name)
		}
	}

	// 2. 模拟首选节点并发达到上限 (ActiveConnections = MaxConcurrency = 2)
	initialBackend.ActiveConnections = 2

	// 此时应当自动溢出 (Spillover) 至次优节点
	spilloverBackend := SelectByHRW(sessionKey, backends)
	if spilloverBackend.ID == initialBackend.ID {
		t.Errorf("首选节点已饱和但未能触发 Spillover 溢出保护")
	}
}

func TestProbeBackend(t *testing.T) {
	// Mock 后端：支持 chat，不支持 responses (返回 404)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/chat/completions" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		if r.URL.Path == "/v1/responses" || r.URL.Path == "/responses" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	prober := NewProber(2 * time.Second)
	backend := &models.Backend{
		BaseURL: mockServer.URL,
	}

	healthy, protos, latency := prober.ProbeBackend(context.Background(), backend)
	if !healthy {
		t.Errorf("ProbeBackend healthy 期望 true, 得到 false")
	}
	if latency < 0 {
		t.Errorf("ProbeBackend latency 期望 >= 0, 得到 %d", latency)
	}
	if len(protos) != 1 || protos[0] != models.ProtocolChat {
		t.Errorf("ProbeBackend 协议识别错误: 期望仅包含 chat, 得到 %v", protos)
	}
}
