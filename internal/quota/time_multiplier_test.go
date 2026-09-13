package quota

import (
	"testing"
	"time"

	"code-gate/internal/models"
)

func TestMatchTimeMultiplier(t *testing.T) {
	rules := []models.TimeMultiplierRule{
		{
			ID:         "rule-night",
			Name:       "夜间空闲特惠 (0.2x)",
			DaysOfWeek: []int{1, 2, 3, 4, 5, 6, 7}, // 每天
			StartTime:  "21:00",
			EndTime:    "09:00",
			Multiplier: 0.2,
			IsEnabled:  true,
		},
		{
			ID:         "rule-workday-peak-am",
			Name:       "工作日早高峰 (1.5x)",
			DaysOfWeek: []int{1, 2, 3, 4, 5}, // 周一至周五
			StartTime:  "10:00",
			EndTime:    "12:00",
			Multiplier: 1.5,
			IsEnabled:  true,
		},
		{
			ID:         "rule-workday-peak-pm",
			Name:       "工作日午后高峰 (1.5x)",
			DaysOfWeek: []int{1, 2, 3, 4, 5}, // 周一至周五
			StartTime:  "15:00",
			EndTime:    "16:00",
			Multiplier: 1.5,
			IsEnabled:  true,
		},
		{
			ID:         "rule-disabled",
			Name:       "停用规则",
			DaysOfWeek: []int{1, 2, 3, 4, 5, 6, 7},
			StartTime:  "13:00",
			EndTime:    "14:00",
			Multiplier: 0.5,
			IsEnabled:  false,
		},
	}

	// 2026-09-11 是星期五 (Friday)
	// 2026-09-12 是星期六 (Saturday)
	// 2026-09-13 是星期日 (Sunday)
	// 2026-09-14 是星期一 (Monday)

	tests := []struct {
		name           string
		t              time.Time
		wantMultiplier float64
		wantRuleID     string
	}{
		{
			name:           "周五 21:00 夜间特惠起始",
			t:              time.Date(2026, 9, 11, 21, 0, 0, 0, time.Local),
			wantMultiplier: 0.2,
			wantRuleID:     "rule-night",
		},
		{
			name:           "周五 23:30 夜间前半夜",
			t:              time.Date(2026, 9, 11, 23, 30, 0, 0, time.Local),
			wantMultiplier: 0.2,
			wantRuleID:     "rule-night",
		},
		{
			name:           "周六 00:00 跨午夜边界",
			t:              time.Date(2026, 9, 12, 0, 0, 0, 0, time.Local),
			wantMultiplier: 0.2,
			wantRuleID:     "rule-night",
		},
		{
			name:           "周六 04:30 跨午夜后半夜",
			t:              time.Date(2026, 9, 12, 4, 30, 0, 0, time.Local),
			wantMultiplier: 0.2,
			wantRuleID:     "rule-night",
		},
		{
			name:           "周六 09:00 跨午夜结束时刻",
			t:              time.Date(2026, 9, 12, 9, 0, 0, 0, time.Local),
			wantMultiplier: 0.2,
			wantRuleID:     "rule-night",
		},
		{
			name:           "周六 09:01 结束之后回退默认基准 1.0x",
			t:              time.Date(2026, 9, 12, 9, 1, 0, 0, time.Local),
			wantMultiplier: 1.0,
			wantRuleID:     "",
		},
		{
			name:           "周一 10:30 工作日早高峰 1.5x",
			t:              time.Date(2026, 9, 14, 10, 30, 0, 0, time.Local),
			wantMultiplier: 1.5,
			wantRuleID:     "rule-workday-peak-am",
		},
		{
			name:           "周六 10:30 周末非工作日不触发 1.5x",
			t:              time.Date(2026, 9, 12, 10, 30, 0, 0, time.Local),
			wantMultiplier: 1.0,
			wantRuleID:     "",
		},
		{
			name:           "周一 15:45 工作日午后高峰 1.5x",
			t:              time.Date(2026, 9, 14, 15, 45, 0, 0, time.Local),
			wantMultiplier: 1.5,
			wantRuleID:     "rule-workday-peak-pm",
		},
		{
			name:           "周一 13:30 处于停用规则时段应回退至 1.0",
			t:              time.Date(2026, 9, 14, 13, 30, 0, 0, time.Local),
			wantMultiplier: 1.0,
			wantRuleID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMult, gotRule := MatchTimeMultiplier(rules, tt.t)
			if gotMult != tt.wantMultiplier {
				t.Errorf("MatchTimeMultiplier() mult = %v, want %v", gotMult, tt.wantMultiplier)
			}
			if tt.wantRuleID == "" {
				if gotRule != nil {
					t.Errorf("MatchTimeMultiplier() rule = %v, want nil", gotRule.ID)
				}
			} else {
				if gotRule == nil || gotRule.ID != tt.wantRuleID {
					t.Errorf("MatchTimeMultiplier() ruleID = %v, want %v", gotRule, tt.wantRuleID)
				}
			}
		})
	}
}

func TestTimeMultiplierManagerThreadSafe(t *testing.T) {
	mgr := GetGlobalMultiplierManager()
	mgr.SetRules([]models.TimeMultiplierRule{
		{
			ID:         "rule-custom",
			Name:       "测试自定义",
			DaysOfWeek: []int{1, 2, 3, 4, 5, 6, 7},
			StartTime:  "00:00",
			EndTime:    "23:59",
			Multiplier: 0.8,
			IsEnabled:  true,
		},
	})

	rules := mgr.GetRules()
	if len(rules) != 1 || rules[0].Multiplier != 0.8 {
		t.Fatalf("GetRules() failed: %v", rules)
	}

	mult, rule := mgr.GetEffectiveMultiplier(time.Now())
	if mult != 0.8 || rule == nil || rule.ID != "rule-custom" {
		t.Fatalf("GetEffectiveMultiplier() failed: mult=%v, rule=%v", mult, rule)
	}
}
