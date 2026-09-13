package quota

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"code-gate/internal/models"
)

// TimeMultiplierManager 全局时段倍率内存管理器
type TimeMultiplierManager struct {
	mu    sync.RWMutex
	rules []models.TimeMultiplierRule
}

var (
	globalMultiplierManager *TimeMultiplierManager
	managerOnce             sync.Once
)

// GetGlobalMultiplierManager 获取全局单例管理器
func GetGlobalMultiplierManager() *TimeMultiplierManager {
	managerOnce.Do(func() {
		globalMultiplierManager = &TimeMultiplierManager{
			rules: []models.TimeMultiplierRule{},
		}
	})
	return globalMultiplierManager
}

// SetRules 线程安全地热更新所有倍率规则
func (m *TimeMultiplierManager) SetRules(rules []models.TimeMultiplierRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = make([]models.TimeMultiplierRule, len(rules))
	copy(m.rules, rules)
}

// GetRules 线程安全地读取当前所有倍率规则
func (m *TimeMultiplierManager) GetRules() []models.TimeMultiplierRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]models.TimeMultiplierRule, len(m.rules))
	copy(res, m.rules)
	return res
}

// GetEffectiveMultiplier 判定指定时间所命中的生效算力倍率
// 若命中有效规则，返回规则设定的 Multiplier 及规则拷贝；若无规则匹配，返回 (1.0, nil)
func (m *TimeMultiplierManager) GetEffectiveMultiplier(t time.Time) (float64, *models.TimeMultiplierRule) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MatchTimeMultiplier(m.rules, t)
}

// MatchTimeMultiplier 纯函数匹配时段倍率，便于单测与复用
func MatchTimeMultiplier(rules []models.TimeMultiplierRule, t time.Time) (float64, *models.TimeMultiplierRule) {
	// ISO 8601 星期：1 (周一) ~ 7 (周日)
	isoDay := int(t.Weekday())
	if isoDay == 0 {
		isoDay = 7
	}
	// 前一天的 ISO 星期 (用于跨午夜到次日清晨的判定)
	prevIsoDay := isoDay - 1
	if prevIsoDay == 0 {
		prevIsoDay = 7
	}

	curMin := t.Hour()*60 + t.Minute()

	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}

		startMin, err1 := parseHHMM(rule.StartTime)
		endMin, err2 := parseHHMM(rule.EndTime)
		if err1 != nil || err2 != nil {
			continue
		}

		// 检查星期是否匹配
		dayMatches := func(day int) bool {
			if len(rule.DaysOfWeek) == 0 {
				return true
			}
			for _, d := range rule.DaysOfWeek {
				if d == day {
					return true
				}
			}
			return false
		}

		if startMin <= endMin {
			// 同一天内区间 (如 10:00 - 12:00)
			if dayMatches(isoDay) && curMin >= startMin && curMin <= endMin {
				ruleCopy := rule
				return rule.Multiplier, &ruleCopy
			}
		} else {
			// 跨午夜区间 (如 21:00 - 09:00)
			// 1. 当天前半夜 (例如 21:00 ~ 23:59)：需满足当天星期匹配且当前时间 >= startMin
			if curMin >= startMin && dayMatches(isoDay) {
				ruleCopy := rule
				return rule.Multiplier, &ruleCopy
			}
			// 2. 跨至次日后半夜 (例如 00:00 ~ 09:00)：需满足前一天星期匹配且当前时间 <= endMin
			if curMin <= endMin && dayMatches(prevIsoDay) {
				ruleCopy := rule
				return rule.Multiplier, &ruleCopy
			}
		}
	}

	return 1.0, nil
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, errors.New("invalid time format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, errors.New("invalid hour")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, errors.New("invalid minute")
	}
	return h*60 + m, nil
}

// GetGlobalTimeMultiplier 快捷获取全局当前时间的算力倍率
func GetGlobalTimeMultiplier(t time.Time) (float64, *models.TimeMultiplierRule) {
	return GetGlobalMultiplierManager().GetEffectiveMultiplier(t)
}
