package api

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// DynamicClientFilter 线程安全客户端过滤规则管理器
type DynamicClientFilter struct {
	mu      sync.RWMutex
	raw     []string
	lowered []string
}

var (
	globalFilter *DynamicClientFilter
	filterOnce   sync.Once
)

// GetGlobalClientFilter 获取全局单例客户端过滤器
func GetGlobalClientFilter(initial []string) *DynamicClientFilter {
	filterOnce.Do(func() {
		globalFilter = NewDynamicClientFilter(initial)
	})
	if initial != nil && len(globalFilter.GetBlockedUAs()) == 0 {
		globalFilter.SetBlockedUAs(initial)
	}
	return globalFilter
}

// NewDynamicClientFilter 创建动态过滤器
func NewDynamicClientFilter(initial []string) *DynamicClientFilter {
	f := &DynamicClientFilter{}
	f.SetBlockedUAs(initial)
	return f
}

// SetBlockedUAs 设置全量黑名单 UA 列表并转换为小写缓存
func (f *DynamicClientFilter) SetBlockedUAs(uas []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.raw = make([]string, 0, len(uas))
	f.lowered = make([]string, 0, len(uas))
	for _, ua := range uas {
		trimmed := strings.TrimSpace(ua)
		if trimmed != "" {
			f.raw = append(f.raw, trimmed)
			f.lowered = append(f.lowered, strings.ToLower(trimmed))
		}
	}
}

// GetBlockedUAs 获取当前原始黑名单列表
func (f *DynamicClientFilter) GetBlockedUAs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	res := make([]string, len(f.raw))
	copy(res, f.raw)
	return res
}

// IsBlocked 判断指定 UA 是否命中黑名单规则
func (f *DynamicClientFilter) IsBlocked(ua string) bool {
	if ua == "" {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	reqUA := strings.ToLower(ua)
	for _, blocked := range f.lowered {
		if strings.Contains(reqUA, blocked) {
			return true
		}
	}
	return false
}

// ClientFilterMiddleware 客户端动态安全过滤中间件
func ClientFilterMiddleware(filter *DynamicClientFilter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if filter != nil && filter.IsBlocked(c.Request.UserAgent()) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"message": "访问被拒绝：检测到未授权或不安全的客户端特征",
					"type":    "forbidden_client",
					"code":    "client_blocked",
				},
			})
			return
		}
		c.Next()
	}
}
