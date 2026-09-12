package quota

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"code-gate/internal/models"
	"code-gate/internal/store"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrDailyQuotaExceeded  = errors.New("已达到今日 Credits 算力配额上限")
	ErrWeeklyQuotaExceeded = errors.New("已达到本周 Credits 算力配额上限")
	ErrRateLimitExceeded   = errors.New("触发每分钟速率限制 (RPM)，请稍后重试")
	ErrModelNotAllowed     = errors.New("当前配额策略未授权访问该模型")
	ErrTimeRangeNotAllowed = errors.New("当前配额策略限制仅在指定时间段内可用")
)

// Engine 双周期弹性配额与限流引擎
type Engine struct {
	rateMap sync.Map // map[uint]*slidingWindow
	mu      sync.Mutex
}

// slidingWindow 简单的内存滑动时间窗口
type slidingWindow struct {
	sync.Mutex
	timestamps []time.Time
}

// NewEngine 创建配额引擎
func NewEngine() *Engine {
	e := &Engine{}
	// 启动定期清理过期限流时间戳的后台任务
	go e.startCleanupLoop(5 * time.Minute)
	return e
}

// CheckQuota 在请求执行前执行严格的配额与权限预检
func (e *Engine) CheckQuota(
	db *gorm.DB,
	userID uint,
	modelName string,
) (*models.GateUserQuota, *models.CreditsWallet, error) {
	if db == nil {
		return nil, nil, errors.New("数据库未连接")
	}

	quota, wallet, err := store.InitOrGetUserQuota(db, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("加载用户配额失败: %w", err)
	}

	// 0. 管理员策略拥有全量模型权限，免除额度与频次限制
	if quota.Policy != nil && quota.Policy.Name == "admin_policy" {
		return quota, wallet, nil
	}

	// 1. 获取该用户生效的限额与策略
	dailyLimit := 50.0
	weeklyLimit := 200.0
	rpm := 60
	var whitelist []string

	if quota.Policy != nil {
		dailyLimit = quota.Policy.DailyCreditsLimit
		weeklyLimit = quota.Policy.WeeklyCreditsLimit
		rpm = quota.Policy.RateLimitRPM
		if len(quota.Policy.ModelWhitelist) > 0 {
			_ = json.Unmarshal(quota.Policy.ModelWhitelist, &whitelist)
		}
	}

	// 个性化配置优先覆盖
	if quota.CustomDailyCredits != nil && *quota.CustomDailyCredits > 0 {
		dailyLimit = *quota.CustomDailyCredits
	}
	if quota.CustomWeeklyCredits != nil && *quota.CustomWeeklyCredits > 0 {
		weeklyLimit = *quota.CustomWeeklyCredits
	} else if quota.CustomDailyCredits != nil {
		// 默认周配额为日配额 4 倍
		weeklyLimit = *quota.CustomDailyCredits * 4.0
	}

	// 2. 模型白名单校验
	if len(whitelist) > 0 {
		allowed := false
		for _, m := range whitelist {
			if m == "*" || m == modelName {
				allowed = true
				break
			}
		}
		if !allowed {
			return quota, wallet, fmt.Errorf("%w: 模型 [%s]", ErrModelNotAllowed, modelName)
		}
	}

	// 3. 可用时间段策略管控 (仅对非管理员策略生效)
	now := time.Now()
	if quota.Policy != nil && len(quota.Policy.TimeRanges) > 0 {
		if err := checkTimeRanges(quota.Policy.TimeRanges, now); err != nil {
			return quota, wallet, err
		}
	}

	// 4. 跨日与跨周重置检测
	e.checkAndResetCycle(db, wallet, now)

	// 5. 双周期剩余 Credits 额度判定
	if wallet.DailyConsumed >= dailyLimit {
		return quota, wallet, fmt.Errorf("%w (已消耗: %.2f / 上限: %.2f)", ErrDailyQuotaExceeded, wallet.DailyConsumed, dailyLimit)
	}
	if wallet.WeeklyConsumed >= weeklyLimit {
		return quota, wallet, fmt.Errorf("%w (已消耗: %.2f / 上限: %.2f)", ErrWeeklyQuotaExceeded, wallet.WeeklyConsumed, weeklyLimit)
	}

	// 6. 内存滑动窗口 RPM 限流校验
	if rpm > 0 && !e.allowRPM(userID, rpm, now) {
		return quota, wallet, fmt.Errorf("%w (上限: %d 次/分钟)", ErrRateLimitExceeded, rpm)
	}

	return quota, wallet, nil
}

// checkAndResetCycle 检查并重置自然日与自然周台账
func (e *Engine) checkAndResetCycle(db *gorm.DB, wallet *models.CreditsWallet, now time.Time) {
	updates := make(map[string]interface{})

	// 判定跨自然日
	if now.Format("2006-01-02") != wallet.LastDailyReset.Format("2006-01-02") {
		wallet.DailyConsumed = 0
		wallet.LastDailyReset = now
		updates["daily_consumed"] = 0
		updates["last_daily_reset"] = now
	}

	// 判定跨自然周 (ISO 8601 周)
	nowYear, nowWeek := now.ISOWeek()
	lastYear, lastWeek := wallet.LastWeeklyReset.ISOWeek()
	if nowYear != lastYear || nowWeek != lastWeek {
		wallet.WeeklyConsumed = 0
		wallet.LastWeeklyReset = now
		updates["weekly_consumed"] = 0
		updates["last_weekly_reset"] = now
	}

	if len(updates) > 0 {
		_ = db.Model(&models.CreditsWallet{}).Where("id = ?", wallet.ID).Updates(updates)
	}
}

// allowRPM 校验每分钟请求滑动窗口
func (e *Engine) allowRPM(userID uint, rpm int, now time.Time) bool {
	val, _ := e.rateMap.LoadOrStore(userID, &slidingWindow{})
	w := val.(*slidingWindow)

	w.Lock()
	defer w.Unlock()

	oneMinAgo := now.Add(-1 * time.Minute)
	valid := w.timestamps[:0]
	for _, t := range w.timestamps {
		if t.After(oneMinAgo) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rpm {
		w.timestamps = valid
		return false
	}

	w.timestamps = append(valid, now)
	return true
}

// startCleanupLoop 定期回收闲置的滑动窗口内存对象
func (e *Engine) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		oneMinAgo := now.Add(-1 * time.Minute)
		e.rateMap.Range(func(key, value interface{}) bool {
			w := value.(*slidingWindow)
			w.Lock()
			if len(w.timestamps) == 0 || w.timestamps[len(w.timestamps)-1].Before(oneMinAgo) {
				e.rateMap.Delete(key)
			}
			w.Unlock()
			return true
		})
	}
}

// checkTimeRanges 校验指定时间是否在允许的时间段列表中
// 支持常规区间 (如 "09:00-18:00") 与跨午夜区间 (如 "22:00-06:00")；若包含 "*" 或列表为空则全天放行
func checkTimeRanges(rawJSON datatypes.JSON, now time.Time) error {
	if len(rawJSON) == 0 {
		return nil
	}
	var ranges []string
	if err := json.Unmarshal(rawJSON, &ranges); err != nil {
		return nil
	}
	if len(ranges) == 0 {
		return nil
	}

	curMin := now.Hour()*60 + now.Minute()
	matched := false

	for _, tr := range ranges {
		tr = strings.TrimSpace(tr)
		if tr == "" || tr == "*" {
			return nil
		}
		parts := strings.Split(tr, "-")
		if len(parts) != 2 {
			continue
		}
		startMin, err1 := parseHourMinute(strings.TrimSpace(parts[0]))
		endMin, err2 := parseHourMinute(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			continue
		}

		if startMin <= endMin {
			// 同一天内区间 (如 09:00-18:00)
			if curMin >= startMin && curMin <= endMin {
				matched = true
				break
			}
		} else {
			// 跨午夜区间 (如 22:00-06:00)
			if curMin >= startMin || curMin <= endMin {
				matched = true
				break
			}
		}
	}

	if !matched {
		return fmt.Errorf("%w: 当前时间 %02d:%02d 不在允许的时间段内 (%v)", ErrTimeRangeNotAllowed, now.Hour(), now.Minute(), ranges)
	}
	return nil
}

func parseHourMinute(s string) (int, error) {
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
