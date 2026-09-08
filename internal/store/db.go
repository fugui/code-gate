package store

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"code-common/backend/gormdb"
	"code-gate/internal/config"
	"code-gate/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	globalDB *gorm.DB
	dbLock   sync.RWMutex
)

// GetDB 获取全局 GORM 实例
func GetDB() *gorm.DB {
	dbLock.RLock()
	defer dbLock.RUnlock()
	return globalDB
}

// SetDB 设置全局 GORM 实例（测试可用）
func SetDB(db *gorm.DB) {
	dbLock.Lock()
	globalDB = db
	dbLock.Unlock()
}

// InitDB 初始化数据库连接、数据表自动迁移与默认种子数据
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gormdb.Connect(cfg.Database, gormdb.Options{
		ServiceName:   "CodeGate",
		SlowLogFile:   "slow_sql.log",
		SlowThreshold: 500 * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 数据库失败: %w", err)
	}

	log.Printf("[CodeGate] 正在执行数据表自动迁移 (AutoMigrate)...")
	err = db.AutoMigrate(
		&models.GateUserQuota{},
		&models.QuotaPolicy{},
		&models.CreditsWallet{},
		&models.Model{},
		&models.Backend{},
		&models.APIKey{},
		&models.AccessLog{},
	)
	if err != nil {
		return nil, fmt.Errorf("数据表自动迁移失败: %w", err)
	}

	// 初始化默认种子数据
	seedDefaults(db, cfg)

	dbLock.Lock()
	globalDB = db
	dbLock.Unlock()

	log.Printf("[CodeGate] 数据库持久层初始化完成")
	return db, nil
}

// seedDefaults 初始化默认策略与初始配置
func seedDefaults(db *gorm.DB, cfg *config.Config) {
	// 1. 初始化 guest 默认配额策略
	var guestPolicy models.QuotaPolicy
	if err := db.Where("name = ?", "guest_policy").First(&guestPolicy).Error; err != nil {
		allModels, _ := json.Marshal([]string{"*"})
		guestPolicy = models.QuotaPolicy{
			Name:               "guest_policy",
			DailyCreditsLimit:  cfg.Defaults.GuestQuota.DailyCredits,
			WeeklyCreditsLimit: cfg.Defaults.GuestQuota.WeeklyCredits,
			RateLimitRPM:       cfg.Defaults.GuestQuota.RateLimitRPM,
			TimeRanges:         datatypes.JSON("[]"),
			ModelWhitelist:     datatypes.JSON(allModels),
		}
		if createErr := db.Create(&guestPolicy).Error; createErr == nil {
			log.Printf("[CodeGate] 已创建默认访客配额策略: guest_policy (日: %.1f, 周: %.1f)",
				guestPolicy.DailyCreditsLimit, guestPolicy.WeeklyCreditsLimit)
		}
	}

	// 2. 初始化开发者 developer 配额策略
	var devPolicy models.QuotaPolicy
	if err := db.Where("name = ?", "developer_policy").First(&devPolicy).Error; err != nil {
		allModels, _ := json.Marshal([]string{"*"})
		devPolicy = models.QuotaPolicy{
			Name:               "developer_policy",
			DailyCreditsLimit:  500.0,
			WeeklyCreditsLimit: 2000.0,
			RateLimitRPM:       120,
			TimeRanges:         datatypes.JSON("[]"),
			ModelWhitelist:     datatypes.JSON(allModels),
		}
		_ = db.Create(&devPolicy).Error
	}

	// 3. 初始化超级管理员/管理员 admin 配额策略
	var adminPolicy models.QuotaPolicy
	if err := db.Where("name = ?", "admin_policy").First(&adminPolicy).Error; err != nil {
		allModels, _ := json.Marshal([]string{"*"})
		adminPolicy = models.QuotaPolicy{
			Name:               "admin_policy",
			DailyCreditsLimit:  100000.0,
			WeeklyCreditsLimit: 500000.0,
			RateLimitRPM:       1000,
			TimeRanges:         datatypes.JSON("[]"),
			ModelWhitelist:     datatypes.JSON(allModels),
		}
		_ = db.Create(&adminPolicy).Error
	}
}

// InitOrGetUserQuota 获取或自动初始化指定用户的 CodeGate 配额记录与算力台账
func InitOrGetUserQuota(db *gorm.DB, userID uint) (*models.GateUserQuota, *models.CreditsWallet, error) {
	var quota models.GateUserQuota
	err := db.Preload("Policy").Where("user_id = ?", userID).First(&quota).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 首次访问：自动绑定 guest 角色
			var guestPolicy models.QuotaPolicy
			var policyID *uint
			if pErr := db.Where("name = ?", "guest_policy").First(&guestPolicy).Error; pErr == nil {
				policyID = &guestPolicy.ID
			}

			quota = models.GateUserQuota{
				UserID:   userID,
				Role:     models.RoleGuest,
				PolicyID: policyID,
			}
			if createErr := db.Create(&quota).Error; createErr != nil {
				return nil, nil, fmt.Errorf("创建用户默认配额失败: %w", createErr)
			}
			if policyID != nil {
				quota.Policy = &guestPolicy
			}
		} else {
			return nil, nil, err
		}
	}

	// 确保钱包记录存在
	var wallet models.CreditsWallet
	wErr := db.Where("user_id = ?", userID).First(&wallet).Error
	if wErr != nil {
		if wErr == gorm.ErrRecordNotFound {
			now := time.Now()
			wallet = models.CreditsWallet{
				UserID:          userID,
				DailyConsumed:   0,
				WeeklyConsumed:  0,
				LastDailyReset:  now,
				LastWeeklyReset: now,
			}
			if cErr := db.Create(&wallet).Error; cErr != nil {
				return nil, nil, fmt.Errorf("创建用户算力钱包失败: %w", cErr)
			}
		} else {
			return nil, nil, wErr
		}
	}

	return &quota, &wallet, nil
}

// EnsureAdminQuota 确保平台超级管理员具备 CodeGate 的 admin 角色与专属充沛配额策略
func EnsureAdminQuota(db *gorm.DB, userID uint) (*models.GateUserQuota, error) {
	var adminPolicy models.QuotaPolicy
	var policyID *uint
	if pErr := db.Where("name = ?", "admin_policy").First(&adminPolicy).Error; pErr == nil {
		policyID = &adminPolicy.ID
	}

	var quota models.GateUserQuota
	err := db.Preload("Policy").Where("user_id = ?", userID).First(&quota).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			quota = models.GateUserQuota{
				UserID:   userID,
				Role:     models.RoleAdmin,
				PolicyID: policyID,
			}
			if cErr := db.Create(&quota).Error; cErr != nil {
				return nil, fmt.Errorf("创建管理员配额记录失败: %w", cErr)
			}
			if policyID != nil {
				quota.Policy = &adminPolicy
			}
			return &quota, nil
		}
		return nil, err
	}

	// 若已存在但角色非 admin，自动同步升级为 admin 角色并关联 admin_policy
	if quota.Role != models.RoleAdmin {
		quota.Role = models.RoleAdmin
		quota.PolicyID = policyID
		if sErr := db.Model(&quota).Updates(map[string]interface{}{
			"role":      models.RoleAdmin,
			"policy_id": policyID,
		}).Error; sErr != nil {
			log.Printf("[CodeGate] 自动同步升级管理员角色失败: %v", sErr)
		}
		if policyID != nil {
			quota.Policy = &adminPolicy
		}
	}
	return &quota, nil
}
