package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ClientFilterMiddleware 客户端动态安全过滤中间件：拦截黑名单 User-Agent 或非法扫描器特征
func ClientFilterMiddleware(blockedUAs []string) gin.HandlerFunc {
	// 预先将黑名单项转换为小写以加快匹配
	lowered := make([]string, 0, len(blockedUAs))
	for _, ua := range blockedUAs {
		trimmed := strings.TrimSpace(strings.ToLower(ua))
		if trimmed != "" {
			lowered = append(lowered, trimmed)
		}
	}

	return func(c *gin.Context) {
		reqUA := strings.ToLower(c.Request.UserAgent())
		if reqUA != "" && len(lowered) > 0 {
			for _, blocked := range lowered {
				if strings.Contains(reqUA, blocked) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error": gin.H{
							"message": "访问被拒绝：检测到未授权或不安全的客户端特征",
							"type":    "forbidden_client",
							"code":    "client_blocked",
						},
					})
					return
				}
			}
		}

		c.Next()
	}
}
