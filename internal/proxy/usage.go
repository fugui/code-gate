package proxy

import (
	"encoding/json"
	"math"
)

// TokenUsage 表示单次请求的真实 Token 消耗统计
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CachedTokens     int `json:"cached_tokens"`
}

// RawUsageResponse 用于解析上游返回的包含 usage 的 JSON 结构
type RawUsageResponse struct {
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

// CalculateCredits 核心计算公式：
// Credits = ((InputTokens * 1.0 + CacheHitTokens * 0.1 + OutputTokens * 5.0) / 1000) * Multiplier
func CalculateCredits(usage TokenUsage, multiplier float64) float64 {
	if multiplier <= 0 {
		multiplier = 1.0
	}

	// 实际未命中的全新输入 Token
	rawInput := usage.PromptTokens - usage.CachedTokens
	if rawInput < 0 {
		rawInput = 0
	}

	rawCost := (float64(rawInput)*1.0 + float64(usage.CachedTokens)*0.1 + float64(usage.CompletionTokens)*5.0) / 1000.0
	credits := rawCost * multiplier

	// 保留 4 位小数并四舍五入
	return math.Round(credits*10000) / 10000
}

// ParseUsageFromChunk 从单条 SSE chunk 字符串中尝试提取 Usage
func ParseUsageFromChunk(chunk []byte) *TokenUsage {
	var resp RawUsageResponse
	if err := json.Unmarshal(chunk, &resp); err != nil {
		return nil
	}
	if resp.Usage == nil {
		return nil
	}

	usage := &TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}
	if resp.Usage.PromptTokensDetails != nil {
		usage.CachedTokens = resp.Usage.PromptTokensDetails.CachedTokens
	}
	return usage
}
