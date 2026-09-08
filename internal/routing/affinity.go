package routing

import (
	"bytes"
	"encoding/json"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"code-gate/internal/models"
	"github.com/gin-gonic/gin"
)

// ExtractSessionID 从请求标头或请求体中提取会话标识
func ExtractSessionID(c *gin.Context, bodyBytes []byte) string {
	// 1. 优先从 Header 提取
	headers := []string{
		"X-Session-ID",
		"X-Conversation-ID",
		"Session-ID",
		"Conversation-ID",
		"X-Chat-ID",
	}
	for _, h := range headers {
		if val := strings.TrimSpace(c.GetHeader(h)); val != "" {
			return val
		}
	}

	// 2. 尝试从 JSON Body 提取
	if len(bodyBytes) > 0 {
		var bodyMap map[string]interface{}
		// 使用轻量 JSON 解码
		d := json.NewDecoder(bytes.NewReader(bodyBytes))
		if err := d.Decode(&bodyMap); err == nil {
			bodyKeys := []string{"session_id", "conversation_id", "chat_id"}
			for _, k := range bodyKeys {
				if val, ok := bodyMap[k]; ok {
					if strVal, ok := val.(string); ok && strings.TrimSpace(strVal) != "" {
						return strings.TrimSpace(strVal)
					}
				}
			}
		}
	}

	return ""
}

type backendScore struct {
	backend *models.Backend
	score   float64
}

// SelectByHRW 使用 HRW (Rendezvous Hashing) 算法进行 KV Cache 亲和性路由与溢出保护
func SelectByHRW(sessionID string, backends []*models.Backend) *models.Backend {
	if len(backends) == 0 {
		return nil
	}
	if len(backends) == 1 {
		return backends[0]
	}

	scores := make([]backendScore, 0, len(backends))

	for _, b := range backends {
		weight := float64(b.Weight)
		if weight <= 0 {
			weight = 1.0
		}

		// 计算确定性哈希
		h := fnv.New64a()
		_, _ = h.Write([]byte(sessionID))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.Itoa(int(b.ID))))
		hashVal := h.Sum64()

		// 归一化至 (0, 1)
		u := float64(hashVal+1) / float64(math.MaxUint64)
		// 标准 HRW 权重公式: -weight / ln(u)
		score := -weight / math.Log(u)

		scores = append(scores, backendScore{
			backend: b,
			score:   score,
		})
	}

	// 按评分从高到低排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// 实施 Spillover 溢出保护：优先选取当前未达并发上限的最高评分节点
	for _, item := range scores {
		b := item.backend
		maxC := atomic.LoadInt32(&b.MaxConcurrency)
		if maxC <= 0 {
			maxC = 10
		}
		active := atomic.LoadInt32(&b.ActiveConnections)
		if active < maxC {
			return b
		}
	}

	// 若所有节点均已饱和，兜底选择评分最高者
	return scores[0].backend
}
