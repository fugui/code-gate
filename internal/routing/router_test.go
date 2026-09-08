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
		selected, err := r.SelectBackend(testModel, models.ProtocolResponses, StrategyWeightedLeastConn)
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
	_, err := r.SelectBackend(chatOnlyModel, models.ProtocolResponses, StrategyWeightedLeastConn)
	if err == nil {
		t.Fatalf("对不支持 responses 的模型选路应当报错，但得到了 nil")
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

	healthy, protos := prober.ProbeBackend(context.Background(), backend)
	if !healthy {
		t.Errorf("ProbeBackend healthy 期望 true, 得到 false")
	}
	if len(protos) != 1 || protos[0] != models.ProtocolChat {
		t.Errorf("ProbeBackend 协议识别错误: 期望仅包含 chat, 得到 %v", protos)
	}
}
