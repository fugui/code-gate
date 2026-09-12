package models

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"

	commonModels "code-common/backend/models"
	"gorm.io/datatypes"
)

// 常量定义
const (
	RoleGuest     = "guest"
	RoleDeveloper = "developer"
	RoleVIP       = "vip"
	RoleAdmin     = "admin"

	ProtocolChat      = "chat"
	ProtocolResponses = "responses"
)

// GateUserQuota 用户在 CodeGate 内部的专属配额角色与限额映射
type GateUserQuota struct {
	ID                  uint         `gorm:"primaryKey" json:"id"`
	UserID              uint         `gorm:"uniqueIndex;not null" json:"user_id"` // 关联 CodeBench 统一用户 ID
	Role                string       `gorm:"size:32;not null;default:'guest'" json:"role"`
	PolicyID            *uint        `gorm:"index" json:"policy_id,omitempty"`
	Policy              *QuotaPolicy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	CustomDailyCredits  *float64     `json:"custom_daily_credits,omitempty"`
	CustomWeeklyCredits *float64     `json:"custom_weekly_credits,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

// QuotaPolicy 配额策略定义
type QuotaPolicy struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Name               string         `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description        string         `gorm:"size:255;default:''" json:"description"`
	DailyCreditsLimit  float64        `gorm:"not null" json:"daily_credits_limit"`
	WeeklyCreditsLimit float64        `gorm:"not null" json:"weekly_credits_limit"` // 默认约为日配额 4 倍
	RateLimitRPM       int            `gorm:"default:60" json:"rate_limit_rpm"`
	TimeRanges         datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"time_ranges"`
	ModelWhitelist     datatypes.JSON `gorm:"type:jsonb;default:'[\"*\"]'" json:"model_whitelist"`
	DefaultModel       string         `gorm:"size:128;default:''" json:"default_model"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// CreditsWallet 用户算力日/周消耗台账
type CreditsWallet struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	DailyConsumed   float64   `gorm:"default:0" json:"daily_consumed"`
	WeeklyConsumed  float64   `gorm:"default:0" json:"weekly_consumed"`
	LastDailyReset  time.Time `json:"last_daily_reset"`
	LastWeeklyReset time.Time `json:"last_weekly_reset"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Model 逻辑模型定义
type Model struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Description  string         `gorm:"size:255;default:''" json:"description"`
	Multiplier   float64        `gorm:"default:1.0" json:"multiplier"` // 算力倍率乘数 (0.5x, 1.0x, 10.0x)
	DefaultModel string         `gorm:"size:128;default:''" json:"default_model"`
	ModelParams  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"model_params"`
	IsEnabled    bool           `gorm:"default:true" json:"is_enabled"`
	Backends     []Backend      `gorm:"foreignKey:ModelID" json:"backends,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// Backend 物理后端实例
type Backend struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	ModelID             uint           `gorm:"index;not null" json:"model_id"`
	Name                string         `gorm:"size:128;default:''" json:"name"`
	BaseURL             string         `gorm:"size:255;not null" json:"base_url"`
	APIKey              string         `gorm:"size:255;default:''" json:"-"`
	Weight              int            `gorm:"default:1" json:"weight"`
	MaxConcurrency      int32          `gorm:"default:10" json:"max_concurrency"`
	DeclaredProtocols   datatypes.JSON `gorm:"type:jsonb;default:'[\"chat\"]'" json:"declared_protocols"`
	DetectedProtocols   datatypes.JSON `gorm:"type:jsonb;default:'[\"chat\"]'" json:"detected_protocols"`
	IsHealthy           bool           `gorm:"default:true" json:"is_healthy"`
	IsEnabled           bool           `gorm:"default:true" json:"is_enabled"`
	LatencyMS           int64          `gorm:"default:0" json:"latency_ms"`
	ConsecutiveFailures int            `gorm:"default:0" json:"consecutive_failures"`
	LastCheckAt         *time.Time     `json:"last_check_at,omitempty"`
	ActiveConnections   int32          `gorm:"-" json:"active_connections"` // 内存无锁并发计数器
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// SystemSetting 全局系统配置键值持久化
type SystemSetting struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SupportsProtocol 检查该物理后端是否支持指定的协议
func (b *Backend) SupportsProtocol(proto string) bool {
	checkJSON := func(dj datatypes.JSON) bool {
		if len(dj) == 0 {
			return false
		}
		var list []string
		if err := json.Unmarshal(dj, &list); err == nil {
			for _, item := range list {
				if strings.EqualFold(item, proto) {
					return true
				}
			}
		}
		return false
	}

	// 优先以自动探测结果为准，其次检查显式声明
	if checkJSON(b.DetectedProtocols) {
		return true
	}
	if checkJSON(b.DeclaredProtocols) {
		return true
	}
	// 默认兜底：若未声明任何协议且请求为 chat，则默认支持
	if strings.EqualFold(proto, ProtocolChat) {
		return true
	}
	return false
}

// AcquireSlot 尝试原子占槽
func (b *Backend) AcquireSlot() bool {
	maxC := atomic.LoadInt32(&b.MaxConcurrency)
	if maxC <= 0 {
		maxC = 10
	}
	for {
		current := atomic.LoadInt32(&b.ActiveConnections)
		if current >= maxC {
			return false
		}
		if atomic.CompareAndSwapInt32(&b.ActiveConnections, current, current+1) {
			return true
		}
	}
}

// ReleaseSlot 释放并发槽位
func (b *Backend) ReleaseSlot() {
	for {
		current := atomic.LoadInt32(&b.ActiveConnections)
		if current <= 0 {
			break
		}
		if atomic.CompareAndSwapInt32(&b.ActiveConnections, current, current-1) {
			break
		}
	}
}

// APIKey 用户专属调用密钥
type APIKey struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	Name          string         `gorm:"size:64;not null" json:"name"`
	RawKey        string         `gorm:"size:128;default:''" json:"raw_key"` // 存储完整明文 Key，便于用户随时查看与复制
	KeyHash       string         `gorm:"size:128;uniqueIndex;not null" json:"-"`
	KeyPrefix     string         `gorm:"size:16;not null" json:"key_prefix"` // 便于前端展示如 sk-abc...
	AllowedModels datatypes.JSON `gorm:"type:jsonb;default:'[\"*\"]'" json:"allowed_models"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	LastUsedAt    *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// AccessLog 全链路访问审计日志
type AccessLog struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"index" json:"user_id"`
	APIKeyID       *uint     `gorm:"index" json:"api_key_id,omitempty"`
	ClientIP       string    `gorm:"size:64" json:"client_ip"`
	UserAgent      string    `gorm:"size:255" json:"user_agent"`
	Path           string    `gorm:"size:128" json:"path"`
	Protocol       string    `gorm:"size:32" json:"protocol"`
	Model          string    `gorm:"size:128;index" json:"model"`
	InputTokens    int       `gorm:"default:0" json:"input_tokens"`
	CacheHitTokens int       `gorm:"default:0" json:"cache_hit_tokens"`
	OutputTokens   int       `gorm:"default:0" json:"output_tokens"`
	CostCredits    float64   `gorm:"default:0" json:"cost_credits"`
	DurationMS     int64     `gorm:"default:0" json:"duration_ms"`
	TTFTMS         int64     `gorm:"default:0" json:"ttft_ms"`
	StatusCode     int       `gorm:"default:200" json:"status_code"`
	ErrorMessage   string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

// User 声明引用公共用户结构
type User = commonModels.User
type Department = commonModels.Department
