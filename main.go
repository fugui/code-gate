package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"code-common/backend/server"
	"code-gate/internal/api"
	"code-gate/internal/audit"
	"code-gate/internal/config"
	"code-gate/internal/proxy"
	"code-gate/internal/quota"
	"code-gate/internal/routing"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	log.Printf("==================================================================")
	log.Printf("             CodeGate（码界）企业大模型统一接入网关                ")
	log.Printf("  Version: %s | Commit: %s | BuildTime: %s", Version, CommitID, BuildTime)
	log.Printf("==================================================================")

	// 如果指定配置不存在，尝试回退使用 config.yaml.example
	actualConfigPath := *configPath
	if _, err := os.Stat(actualConfigPath); os.IsNotExist(err) {
		if _, errEx := os.Stat("config.yaml.example"); errEx == nil {
			log.Printf("[CodeGate] 未检测到 %s，自动回退使用 config.yaml.example 作为运行时配置", actualConfigPath)
			actualConfigPath = "config.yaml.example"
		}
	}

	cfg, err := config.Load(actualConfigPath)
	if err != nil {
		log.Fatalf("[CodeGate] 加载配置文件失败: %v", err)
	}

	// 1. 初始化 PostgreSQL 数据库
	_, err = store.InitDB(cfg)
	if err != nil {
		log.Fatalf("[CodeGate] 数据库初始化失败: %v", err)
	}

	// 2. 初始化核心代理客户端与智能调度组件
	proxyClient := proxy.NewClient(cfg.Server.WriteTimeout)
	router := routing.NewRouter()
	quotaEngine := quota.NewEngine()

	// 3. 启动后台协议与健康探活协程
	prober := routing.NewProber(5 * time.Second)
	prober.Start(30 * time.Second)

	// 4. 启动审计日志定时轮转清理协程
	cleanupStopChan := audit.StartLogCleanupTask(store.GetDB(), 7, 24*time.Hour)

	// 5. 基于 code-common/backend/server 脚手架启动微服务
	serverOpts := server.Options{
		ServiceName:       "Code-Gate",
		Prefix:            cfg.Server.Prefix,
		Port:              cfg.Server.Port,
		GinLog:            cfg.Server.GinLog,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		FrontendFS:        &frontendFS,
		FrontendDistPath:  "frontend/dist",
		ExtraNoRoute: func(c *gin.Context) bool {
			if strings.HasPrefix(c.Request.URL.Path, "/v1") {
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"message": "API endpoint not found",
						"type":    "invalid_request_error",
						"code":    "not_found",
					},
				})
				return true
			}
			return false
		},
		RegisterRoutes: func(r *gin.Engine) {
			// 免密健康探针接口
			r.GET("/health", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"status":  "healthy",
					"service": "code-gate",
					"version": Version,
				})
			})

			// 统一鉴权与核心网关路由
			v1Group := r.Group("/v1")
			v1Group.Use(api.UnifiedAuthMiddleware(func() string {
				return cfg.Auth.JWTSecret
			}))
			{
				// OpenAI 协议核心直通接口
				v1Group.GET("/models", api.HandleListModels)
				v1Group.POST("/chat/completions", api.HandleChatCompletions(proxyClient, router, quotaEngine))
				v1Group.POST("/responses", api.HandleResponses(proxyClient, router, quotaEngine))

				// 用户个人配额与 API Key 管理
				v1Group.GET("/user/profile", api.HandleGetUserProfile)
				v1Group.GET("/user/keys", api.HandleListUserKeys)
				v1Group.POST("/user/keys", api.HandleCreateUserKey)
				v1Group.DELETE("/user/keys/:id", api.HandleDeleteUserKey)

				// 管理员受保护管理路由组
				adminGroup := v1Group.Group("/admin")
				adminGroup.Use(api.RequireAdmin())
				{
					// 用户与配额管理
					adminGroup.GET("/users", api.HandleAdminListUsers)
					adminGroup.PUT("/users/:id/quota", api.HandleAdminUpdateUserQuota)

					// 策略定义
					adminGroup.GET("/policies", api.HandleAdminListPolicies)
					adminGroup.POST("/policies", api.HandleAdminSavePolicy)

					// 逻辑模型管理
					adminGroup.POST("/models", api.HandleAdminSaveModel)
					adminGroup.DELETE("/models/:id", api.HandleAdminDeleteModel)

					// 物理后端实例管理与探活状态
					adminGroup.GET("/backends", api.HandleAdminListBackends)
					adminGroup.POST("/backends", api.HandleAdminSaveBackend)
					adminGroup.DELETE("/backends/:id", api.HandleAdminDeleteBackend)

					// 审计日志检索
					adminGroup.GET("/logs", api.HandleAdminListLogs)
				}
			}
		},
		OnShutdown: func(ctx context.Context) {
			log.Printf("[CodeGate] 正在停止后台探针与释放资源...")
			prober.Stop()
			close(cleanupStopChan)
			log.Printf("[CodeGate] 优雅停机处理完成")
		},
	}

	if err := server.Run(serverOpts); err != nil {
		fmt.Fprintf(os.Stderr, "服务异常终止: %v\n", err)
		os.Exit(1)
	}
}
