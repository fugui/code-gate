package proxy

import (
	"testing"
)

func TestCalculateCredits(t *testing.T) {
	tests := []struct {
		name       string
		usage      TokenUsage
		multiplier float64
		want       float64
	}{
		{
			name: "标准请求无缓存命中 (倍率 1.0)",
			usage: TokenUsage{
				PromptTokens:     1000,
				CachedTokens:     0,
				CompletionTokens: 500,
			},
			multiplier: 1.0,
			want:       3.5, // (1000*1 + 0*0.1 + 500*5) / 1000 = 3.5
		},
		{
			name: "长会话缓存命中 80% (Prompt Cache 优惠)",
			usage: TokenUsage{
				PromptTokens:     1000,
				CachedTokens:     800,
				CompletionTokens: 500,
			},
			multiplier: 1.0,
			want:       2.78, // (200*1 + 800*0.1 + 500*5) / 1000 = (200 + 80 + 2500)/1000 = 2.78
		},
		{
			name: "旗舰大模型高倍率 10.0",
			usage: TokenUsage{
				PromptTokens:     1000,
				CachedTokens:     0,
				CompletionTokens: 500,
			},
			multiplier: 10.0,
			want:       35.0, // 3.5 * 10 = 35.0
		},
		{
			name: "轻量快速模型低倍率 0.5",
			usage: TokenUsage{
				PromptTokens:     1000,
				CachedTokens:     0,
				CompletionTokens: 500,
			},
			multiplier: 0.5,
			want:       1.75, // 3.5 * 0.5 = 1.75
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateCredits(tt.usage, tt.multiplier)
			if got != tt.want {
				t.Errorf("CalculateCredits() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseUsageFromChunk(t *testing.T) {
	chunk := []byte(`{"id":"chatcmpl-123","object":"chat.completion.chunk","usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"prompt_tokens_details":{"cached_tokens":30}}}`)
	usage := ParseUsageFromChunk(chunk)
	if usage == nil {
		t.Fatalf("ParseUsageFromChunk() returned nil")
	}
	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d, want 100", usage.PromptTokens)
	}
	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d, want 50", usage.CompletionTokens)
	}
	if usage.CachedTokens != 30 {
		t.Errorf("CachedTokens = %d, want 30", usage.CachedTokens)
	}
}
