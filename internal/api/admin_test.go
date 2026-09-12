package api

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	var devPolicy models.QuotaPolicy
	if err := db.Where("name = ?", "developer_policy").First(&devPolicy).Error; err != nil {
		t.Fatalf("查询 developer_policy 失败: %v", err)
	}

	reqBody := fmt.Sprintf(`{
		"policy_id": %d,
		"is_custom": true,
		"custom_daily_credits": 120.0
	}`, devPolicy.ID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/admin/users/99999/quota", bytes.NewReader([]byte(reqBody)))
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

	if updatedQuota.PolicyID == nil {
		t.Fatalf("配额策略 ID 为 nil")
	}
	if *updatedQuota.PolicyID != devPolicy.ID {
		t.Errorf("配额策略 ID 期望 %d, 实际得到: %d", devPolicy.ID, *updatedQuota.PolicyID)
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
	if profResp.Data.PolicyName != "admin_policy" {
		t.Errorf("超级管理员配额策略期望自动关联 admin_policy, 实际为 %s", profResp.Data.PolicyName)
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

func TestAdminPolicySafetyDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接数据库，跳过测试")
		return
	}

	// 1. 创建测试策略
	policy := models.QuotaPolicy{
		Name:               "test_bound_policy",
		DailyCreditsLimit:  100,
		WeeklyCreditsLimit: 400,
	}
	_ = db.Create(&policy)
	defer db.Delete(&policy)

	// 2. 绑定一个用户
	testUID := uint(88888)
	quota := models.GateUserQuota{
		UserID:   testUID,
		PolicyID: &policy.ID,
	}
	_ = db.Create(&quota)
	defer db.Delete(&quota)

	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	adminGroup.DELETE("/policies/:id", HandleAdminDeletePolicy)

	// 尝试删除被绑定的策略，期望返回 400
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/policies/%d", policy.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望返回 400 拒绝删除有用户绑定的策略，实际返回 %d", w.Code)
	}

	// 解绑用户后再次删除，期望成功 200
	db.Delete(&quota)
	req2, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/policies/%d", policy.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("解绑后删除策略期望 200，实际返回 %d", w2.Code)
	}
}

func TestAdminModelAndBackendLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接数据库，跳过测试")
		return
	}

	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	adminGroup.GET("/models", HandleAdminListModels)
	adminGroup.POST("/models", HandleAdminSaveModel)
	adminGroup.PATCH("/models/:id/toggle", HandleAdminToggleModel)
	adminGroup.DELETE("/models/:id", HandleAdminDeleteModel)

	// 1. 创建模型并附带首个后端
	createReq := `{"name":"test-auto-model","multiplier":2.0,"initial_backend":{"base_url":"http://192.168.56.18:8000","weight":10,"max_concurrency":20}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/admin/models", bytes.NewReader([]byte(createReq)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("创建模型接口失败: %d, body: %s", w.Code, w.Body.String())
	}

	var m models.Model
	if err := db.Preload("Backends").Where("name = ?", "test-auto-model").First(&m).Error; err != nil {
		t.Fatalf("数据库中未找到新模型: %v", err)
	}
	defer db.Delete(&m)
	if len(m.Backends) != 1 {
		t.Fatalf("期望自动创建 1 个物理后端，实际为 %d", len(m.Backends))
	}
	defer db.Delete(&m.Backends[0])

	// 2. 更新模型（包含修改模型标识 Name 与倍率）
	updateReq := fmt.Sprintf(`{"id":%d,"name":"test-auto-model-renamed","description":"更新说明","multiplier":3.0}`, m.ID)
	wUpdate := httptest.NewRecorder()
	reqUpdate, _ := http.NewRequest(http.MethodPost, "/admin/models", bytes.NewReader([]byte(updateReq)))
	reqUpdate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wUpdate, reqUpdate)
	if wUpdate.Code != http.StatusOK {
		t.Fatalf("更新模型失败: %d, body: %s", wUpdate.Code, wUpdate.Body.String())
	}

	var mRenamed models.Model
	if err := db.First(&mRenamed, m.ID).Error; err != nil {
		t.Fatalf("查询重命名后模型失败: %v", err)
	}
	if mRenamed.Name != "test-auto-model-renamed" {
		t.Errorf("期望模型标识更新为 test-auto-model-renamed, 实际为: %s", mRenamed.Name)
	}
	if mRenamed.Multiplier != 3.0 {
		t.Errorf("期望 multiplier 更新为 3.0, 实际为: %v", mRenamed.Multiplier)
	}

	// 3. 切换模型状态
	wToggle := httptest.NewRecorder()
	reqToggle, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("/admin/models/%d/toggle", m.ID), nil)
	r.ServeHTTP(wToggle, reqToggle)
	if wToggle.Code != http.StatusOK {
		t.Fatalf("切换模型状态失败: %d", wToggle.Code)
	}

	var mUpdated models.Model
	db.First(&mUpdated, m.ID)
	if mUpdated.IsEnabled != false {
		t.Errorf("期望切换后 is_enabled=false, 实际为 %v", mUpdated.IsEnabled)
	}

	// 3. 删除模型及其后端
	wDel := httptest.NewRecorder()
	reqDel, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/models/%d", m.ID), nil)
	r.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("删除模型失败: %d", wDel.Code)
	}
}

func TestAdminSystemConfigAndHotReload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接数据库，跳过测试")
		return
	}

	_ = db
	filter := GetGlobalClientFilter([]string{"sqlmap"})
	SetRuntimeServerConfig("30m", "30m", "120s", 1048576)

	r := gin.New()
	adminGroup := r.Group("/admin")
	adminGroup.Use(func(c *gin.Context) {
		c.Set(auth.ContextIsAdmin, true)
		c.Next()
	})
	adminGroup.GET("/config/system", HandleAdminGetSystemConfig)
	adminGroup.PUT("/config/system", HandleAdminUpdateSystemConfig)

	// 1. 获取系统配置
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest(http.MethodGet, "/admin/config/system", nil)
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("获取配置失败: %d", wGet.Code)
	}

	// 2. 更新配置热重载
	updateReq := `{"blocked_user_agents":["sqlmap","test-blocked-bot"]}`
	wPut := httptest.NewRecorder()
	reqPut, _ := http.NewRequest(http.MethodPut, "/admin/config/system", bytes.NewReader([]byte(updateReq)))
	reqPut.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wPut, reqPut)

	if wPut.Code != http.StatusOK {
		t.Fatalf("更新配置失败: %d", wPut.Code)
	}

	if !filter.IsBlocked("Test-Blocked-Bot v1.0") {
		t.Errorf("期望热重载后能够阻断 Test-Blocked-Bot")
	}
}
