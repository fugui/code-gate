package quota

import (
	"testing"
	"time"

	"code-gate/internal/config"
	"code-gate/internal/models"
	"code-gate/internal/store"
)

func TestDualCycleQuotaEngine(t *testing.T) {
	cfg, err := config.Load("../../config.yaml.example")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	db, err := store.InitDB(cfg)
	if err != nil {
		t.Skipf("无法连接本地 PostgreSQL 测试库，跳过配额集成测试: %v", err)
		return
	}

	engine := NewEngine()
	testUserID := uint(88888)

	// 清理旧测试数据
	_ = db.Where("user_id = ?", testUserID).Delete(&models.GateUserQuota{})
	_ = db.Where("user_id = ?", testUserID).Delete(&models.CreditsWallet{})

	// 1. 初次请求：应自动绑定 guest 配额并放行
	quota, wallet, err := engine.CheckQuota(db, testUserID, "deepseek-v3")
	if err != nil {
		t.Fatalf("首次请求配额校验失败: %v", err)
	}
	if quota.Role != models.RoleGuest {
		t.Errorf("首次访问用户角色不正确: 期望 guest, 获得 %s", quota.Role)
	}

	// 2. 模拟单日消耗达上限 (50 Credits)
	_ = db.Model(&models.CreditsWallet{}).Where("user_id = ?", testUserID).Update("daily_consumed", 50.0)
	_, _, err = engine.CheckQuota(db, testUserID, "deepseek-v3")
	if err == nil {
		t.Fatalf("单日消耗打满时应当拦截，但未返回错误")
	}

	// 恢复日用量，测试单周消耗打满 (200 Credits)
	_ = db.Model(&models.CreditsWallet{}).Where("user_id = ?", testUserID).Updates(map[string]interface{}{
		"daily_consumed":  10.0,
		"weekly_consumed": 200.0,
	})
	_, _, err = engine.CheckQuota(db, testUserID, "deepseek-v3")
	if err == nil {
		t.Fatalf("单周消耗打满时应当拦截，但未返回错误")
	}

	// 3. 测试自然日重置
	wallet.LastDailyReset = time.Now().Add(-25 * time.Hour)
	engine.checkAndResetCycle(db, wallet, time.Now())
	if wallet.DailyConsumed != 0 {
		t.Errorf("跨自然日后日消耗量应重置为 0，当前为 %f", wallet.DailyConsumed)
	}

	// 清理测试数据
	_ = db.Where("user_id = ?", testUserID).Delete(&models.GateUserQuota{})
	_ = db.Where("user_id = ?", testUserID).Delete(&models.CreditsWallet{})
}
