package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"code-common/backend/auth"
	"code-gate/internal/config"
	"code-gate/internal/models"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
)

func TestAdminQuotaManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过管理员接口测试: %v", err)
		return
	}

	testTargetUID := uint(99999)

	r := gin.New()
	adminGroup := r.Group("/admin")
	// 模拟管理员已通过认证
	adminGroup.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	adminGroup.Use(RequireAdmin())
	{
		adminGroup.GET("/users", HandleAdminListUsers)
		adminGroup.PUT("/users/:id/quota", HandleAdminUpdateUserQuota)
		adminGroup.POST("/policies", HandleAdminSavePolicy)
	}

	// 1. 管理员调整用户配额：提权为 developer，指定日额度 120 (应自动设定周额度 480)
	reqBody := []byte(`{
		"role": "developer",
		"custom_daily_credits": 120.0
	}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/admin/users/99999/quota", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("调整用户配额接口失败: %d, body: %s", w.Code, w.Body.String())
	}

	// 2. 验证数据库中用户配额记录已更新
	var updatedQuota models.GateUserQuota
	if err := db.Where("user_id = ?", testTargetUID).First(&updatedQuota).Error; err != nil {
		t.Fatalf("查询已更新的配额记录失败: %v", err)
	}

	if updatedQuota.Role != models.RoleDeveloper {
		t.Errorf("配额角色期望 developer, 实际得到: %s", updatedQuota.Role)
	}
	if updatedQuota.CustomDailyCredits == nil || *updatedQuota.CustomDailyCredits != 120.0 {
		t.Errorf("自定义日配额期望 120.0, 实际得到: %v", updatedQuota.CustomDailyCredits)
	}
	if updatedQuota.CustomWeeklyCredits == nil || *updatedQuota.CustomWeeklyCredits != 480.0 {
		t.Errorf("自定义周配额期望自动联动为 480.0, 实际得到: %v", updatedQuota.CustomWeeklyCredits)
	}

	// 3. 测试非管理员访问应被拦截 403
	forbiddenRouter := gin.New()
	forbiddenRouter.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, false)
		c.Next()
	})
	forbiddenRouter.Use(RequireAdmin())
	forbiddenRouter.GET("/admin/users", HandleAdminListUsers)

	fw := httptest.NewRecorder()
	freq, _ := http.NewRequest(http.MethodGet, "/admin/users", nil)
	forbiddenRouter.ServeHTTP(fw, freq)

	if fw.Code != http.StatusForbidden {
		t.Errorf("非管理员访问期望 403 Forbidden, 实际得到 %d", fw.Code)
	}

	// 清理测试数据
	_ = db.Where("user_id = ?", testTargetUID).Delete(&models.GateUserQuota{})
	_ = db.Where("user_id = ?", testTargetUID).Delete(&models.CreditsWallet{})
}

func TestAdminSavePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, _ := config.Load("../../config.yaml.example")
	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过策略测试: %v", err)
		return
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	r.POST("/admin/policies", HandleAdminSavePolicy)

	policyReq := []byte(`{
		"name": "vip_project_policy",
		"daily_credits_limit": 1000.0,
		"rate_limit_rpm": 200
	}`)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/admin/policies", bytes.NewReader(policyReq))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("创建策略失败: %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data models.QuotaPolicy `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Data.WeeklyCreditsLimit != 4000.0 {
		t.Errorf("周配额期望自动设置为日配额4倍 (4000.0), 实际得到 %f", resp.Data.WeeklyCreditsLimit)
	}

	// 清理策略
	_ = db.Where("name = ?", "vip_project_policy").Delete(&models.QuotaPolicy{})
}
