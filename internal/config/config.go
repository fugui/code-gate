package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	commonModels "code-common/backend/models"
	"gopkg.in/yaml.v3"
)

// ServerConfig 定义 HTTP 服务参数
type ServerConfig struct {
	Port           string        `yaml:"port"`
	Prefix         string        `yaml:"prefix"`
	GinLog         bool          `yaml:"gin_log"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
}

// GuestQuotaConfig 定义访客默认配额
type GuestQuotaConfig struct {
	DailyCredits  float64 `yaml:"daily_credits"`
	WeeklyCredits float64 `yaml:"weekly_credits"`
	RateLimitRPM  int     `yaml:"rate_limit_rpm"`
}

// DefaultsConfig 定义系统全局默认项
type DefaultsConfig struct {
	DefaultModel string           `yaml:"default_model"`
	GuestQuota   GuestQuotaConfig `yaml:"guest_quota"`
}

// Config CodeGate 全局配置结构体
type Config struct {
	Server   ServerConfig                `yaml:"server"`
	Database commonModels.DatabaseConfig `yaml:"database"`
	Auth     commonModels.AuthConfig     `yaml:"auth"`
	Defaults DefaultsConfig              `yaml:"defaults"`
}

var (
	globalConfig *Config
	configLock   sync.RWMutex
)

// Get 返回全局配置快照
func Get() *Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return globalConfig
}

// Load 从指定路径加载 YAML 配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败 (%s): %w", path, err)
	}

	// 临时结构用于解析 time.Duration 字符串
	var raw struct {
		Server struct {
			Port           string `yaml:"port"`
			Prefix         string `yaml:"prefix"`
			GinLog         bool   `yaml:"gin_log"`
			ReadTimeout    string `yaml:"read_timeout"`
			WriteTimeout   string `yaml:"write_timeout"`
			IdleTimeout    string `yaml:"idle_timeout"`
			MaxHeaderBytes int    `yaml:"max_header_bytes"`
		} `yaml:"server"`
		Database commonModels.DatabaseConfig `yaml:"database"`
		Auth     commonModels.AuthConfig     `yaml:"auth"`
		Defaults DefaultsConfig              `yaml:"defaults"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析配置文件 YAML 失败: %w", err)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:           raw.Server.Port,
			Prefix:         raw.Server.Prefix,
			GinLog:         raw.Server.GinLog,
			MaxHeaderBytes: raw.Server.MaxHeaderBytes,
		},
		Database: raw.Database,
		Auth:     raw.Auth,
		Defaults: raw.Defaults,
	}

	if raw.Server.ReadTimeout != "" {
		if d, err := time.ParseDuration(raw.Server.ReadTimeout); err == nil {
			cfg.Server.ReadTimeout = d
		}
	}
	if raw.Server.WriteTimeout != "" {
		if d, err := time.ParseDuration(raw.Server.WriteTimeout); err == nil {
			cfg.Server.WriteTimeout = d
		}
	}
	if raw.Server.IdleTimeout != "" {
		if d, err := time.ParseDuration(raw.Server.IdleTimeout); err == nil {
			cfg.Server.IdleTimeout = d
		}
	}

	// 默认兜底参数
	if cfg.Server.Port == "" {
		cfg.Server.Port = ":8088"
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Minute
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 30 * time.Minute
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 120 * time.Second
	}
	if cfg.Server.MaxHeaderBytes == 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20 // 1MB
	}
	if cfg.Defaults.GuestQuota.DailyCredits <= 0 {
		cfg.Defaults.GuestQuota.DailyCredits = 50.0
	}
	if cfg.Defaults.GuestQuota.WeeklyCredits <= 0 {
		cfg.Defaults.GuestQuota.WeeklyCredits = cfg.Defaults.GuestQuota.DailyCredits * 4.0
	}
	if cfg.Defaults.GuestQuota.RateLimitRPM <= 0 {
		cfg.Defaults.GuestQuota.RateLimitRPM = 30
	}
	if cfg.Defaults.DefaultModel == "" {
		cfg.Defaults.DefaultModel = "deepseek-v3"
	}

	configLock.Lock()
	globalConfig = cfg
	configLock.Unlock()

	return cfg, nil
}
