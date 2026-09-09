package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClientFilterMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	blocked := []string{"sqlmap", "nikto", "acunetix", "bad-bot"}
	filter := NewDynamicClientFilter(blocked)
	r.Use(ClientFilterMiddleware(filter))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 1. 正常客户端放行
	req1, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("正常客户端期望 200，实际返回 %d", w1.Code)
	}

	// 2. 空 User-Agent 放行
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("空 User-Agent 期望 200，实际返回 %d", w2.Code)
	}

	// 3. 拦截黑名单 sqlmap
	req3, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req3.Header.Set("User-Agent", "sqlmap/1.5.2#stable (http://sqlmap.org)")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Errorf("sqlmap 期望 403，实际返回 %d", w3.Code)
	}

	// 4. 拦截大小写变体 NikTo
	req4, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req4.Header.Set("User-Agent", "Mozilla/5.0 (NikTo Scanner v2)")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusForbidden {
		t.Errorf("Nikto 扫描器期望 403，实际返回 %d", w4.Code)
	}

	// 5. 动态热重载：新增拦截 custom-crawler
	filter.SetBlockedUAs(append(filter.GetBlockedUAs(), "custom-crawler"))
	req5, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req5.Header.Set("User-Agent", "Custom-Crawler/1.0")
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusForbidden {
		t.Errorf("热重载后的 custom-crawler 期望 403，实际返回 %d", w5.Code)
	}
}
