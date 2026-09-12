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

	// 1. 初次请求：应自动绑定 guest_policy 配额并放行
	quota, wallet, err := engine.CheckQuota(db, testUserID, "deepseek-v3")
	if err != nil {
		t.Fatalf("首次请求配额校验失败: %v", err)
	}
	if quota.Policy == nil || quota.Policy.Name != "guest_policy" {
		t.Errorf("首次访问用户配额策略不正确: 期望 guest_policy, 实际为 %+v", quota.Policy)
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

func TestCheckTimeRanges(t *testing.T) {
	// 1. 全天通配符或空
	if err := checkTimeRanges([]byte(`["*"]`), time.Now()); err != nil {
		t.Errorf("通配符 * 应当放行: %v", err)
	}
	if err := checkTimeRanges([]byte(`[]`), time.Now()); err != nil {
		t.Errorf("空时间段应当放行: %v", err)
	}

	// 2. 常规时段: 09:00 - 18:00
	rangesNormal := []byte(`["09:00-18:00"]`)
	tIn := time.Date(2026, 9, 8, 14, 30, 0, 0, time.Local)
	tOutEarly := time.Date(2026, 9, 8, 8, 59, 0, 0, time.Local)
	tOutLate := time.Date(2026, 9, 8, 18, 01, 0, 0, time.Local)

	if err := checkTimeRanges(rangesNormal, tIn); err != nil {
		t.Errorf("14:30 应当在 09:00-18:00 内放行: %v", err)
	}
	if err := checkTimeRanges(rangesNormal, tOutEarly); err == nil {
		t.Errorf("08:59 不在 09:00-18:00 内应当拦截")
	}
	if err := checkTimeRanges(rangesNormal, tOutLate); err == nil {
		t.Errorf("18:01 不在 09:00-18:00 内应当拦截")
	}

	// 3. 跨午夜时段: 22:00 - 06:00
	rangesMidnight := []byte(`["22:00-06:00"]`)
	tNight := time.Date(2026, 9, 8, 23, 15, 0, 0, time.Local)
	tEarlyMorning := time.Date(2026, 9, 8, 5, 45, 0, 0, time.Local)
	tDaytime := time.Date(2026, 9, 8, 12, 0, 0, 0, time.Local)

	if err := checkTimeRanges(rangesMidnight, tNight); err != nil {
		t.Errorf("23:15 应当在 22:00-06:00 内放行: %v", err)
	}
	if err := checkTimeRanges(rangesMidnight, tEarlyMorning); err != nil {
		t.Errorf("05:45 应当在 22:00-06:00 内放行: %v", err)
	}
	if err := checkTimeRanges(rangesMidnight, tDaytime); err == nil {
		t.Errorf("12:00 不在 22:00-06:00 内应当拦截")
	}
}
