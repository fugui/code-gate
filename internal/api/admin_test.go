package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestSuperAdminAutoGrantAndAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	secret := "ABCDEFGHIJKLMNOPQRSTVUWXYZ0987654321"
	cfg, err := config.Load("../../config.yaml")
	if err != nil {
		cfg, _ = config.Load("../../config.yaml.example")
	}
	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过测试: %v", err)
		return
	}

	r := gin.New()
	v1 := r.Group("/v1")
	v1.Use(UnifiedAuthMiddleware(func() string {
		return secret
	}))
	{
		v1.GET("/user/profile", HandleGetUserProfile)

		admin := v1.Group("/admin")
		admin.Use(RequireAdmin())
		{
			admin.GET("/users", HandleAdminListUsers)
		}
	}

	// 1. 生成普通用户 Token
	normalToken, _ := auth.GenerateToken(8888, "normal", "normal@example.com", "Normal User", false, []string{"user"}, secret, 1*time.Hour)

	// 2. 生成 CodeBench 超级管理员 Token (roles: ["super_admin"], is_admin: true)
	superAdminToken, _ := auth.GenerateToken(9999, "superadmin", "admin@company.com", "Super Admin", true, []string{"super_admin", "admin"}, secret, 1*time.Hour)

	// 3. 普通用户访问 /v1/admin/users 应被拒绝 403
	wNorm := httptest.NewRecorder()
	reqNorm, _ := http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	reqNorm.Header.Set("Authorization", "Bearer "+normalToken)
	r.ServeHTTP(wNorm, reqNorm)
	if wNorm.Code != http.StatusForbidden {
		t.Errorf("普通用户访问管理接口期望返回 403, 实际获得: %d", wNorm.Code)
	}

	// 4. 超级管理员访问 /v1/user/profile 应返回 200，并且 is_admin 为 true，角色为 admin
	wAdminProf := httptest.NewRecorder()
	reqAdminProf, _ := http.NewRequest(http.MethodGet, "/v1/user/profile", nil)
	reqAdminProf.Header.Set("Authorization", "Bearer "+superAdminToken)
	r.ServeHTTP(wAdminProf, reqAdminProf)
	if wAdminProf.Code != http.StatusOK {
		t.Fatalf("超级管理员访问个人配额失败: %d, body: %s", wAdminProf.Code, wAdminProf.Body.String())
	}

	var profResp struct {
		Data UserProfileResponse `json:"data"`
	}
	_ = json.Unmarshal(wAdminProf.Body.Bytes(), &profResp)
	if !profResp.Data.IsAdmin {
		t.Errorf("超级管理员访问个人配额期望 is_admin 为 true, 实际为 false")
	}
	if profResp.Data.Role != models.RoleAdmin {
		t.Errorf("超级管理员角色期望自动提升为 admin, 实际为 %s", profResp.Data.Role)
	}

	// 5. 超级管理员访问 /v1/admin/users 应成功返回 200
	wAdmin := httptest.NewRecorder()
	reqAdmin, _ := http.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+superAdminToken)
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("超级管理员访问管理接口期望返回 200, 实际获得: %d, body: %s", wAdmin.Code, wAdmin.Body.String())
	}

	// 清理测试用户配额记录
	_ = db.Where("user_id IN ?", []uint{8888, 9999}).Delete(&models.GateUserQuota{})
}

func TestHandleAdminGetDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过大屏接口测试: %v", err)
		return
	}

	// 插入一条模拟访问日志
	mockLog := models.AccessLog{
		UserID:      1001,
		ClientIP:    "127.0.0.1",
		UserAgent:   "Go-Test",
		Path:        "/v1/chat/completions",
		Protocol:    "chat",
		Model:       "deepseek-v3",
		CostCredits: 1.5,
		DurationMS:  120,
		StatusCode:  200,
		CreatedAt:   time.Now(),
	}
	_ = db.Create(&mockLog)
	defer db.Delete(&mockLog)

	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	adminGroup.Use(RequireAdmin())
	adminGroup.GET("/dashboard", HandleAdminGetDashboard)

	req, _ := http.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /admin/dashboard 期望 200，实际返回 %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data DashboardDataDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析大屏响应 JSON 失败: %v", err)
	}

	if len(resp.Data.HourlyTrends) != 24 {
		t.Errorf("24 小时趋势点数量期望为 24，实际为 %d", len(resp.Data.HourlyTrends))
	}
	if resp.Data.Summary.TotalRequests <= 0 {
		t.Errorf("总请求数应大于 0")
	}
}

