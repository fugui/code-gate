package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"code-common/backend/server"
	"code-gate/internal/api"
	"code-gate/internal/config"
	"code-gate/internal/proxy"
	"code-gate/internal/store"
	"github.com/gin-gonic/gin"
)

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

	// 2. 初始化核心代理客户端
	proxyClient := proxy.NewClient(cfg.Server.WriteTimeout)

	// 3. 基于 code-common/backend/server 脚手架启动微服务
	serverOpts := server.Options{
		ServiceName:       "Code-Gate",
		Prefix:            cfg.Server.Prefix,
		Port:              cfg.Server.Port,
		GinLog:            cfg.Server.GinLog,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
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
				v1Group.GET("/models", api.HandleListModels)
				v1Group.POST("/chat/completions", api.HandleChatCompletions(proxyClient))
			}
		},
		OnShutdown: func(ctx context.Context) {
			log.Printf("[CodeGate] 正在执行优雅停机与资源释放...")
		},
	}

	if err := server.Run(serverOpts); err != nil {
		fmt.Fprintf(os.Stderr, "服务异常终止: %v\n", err)
		os.Exit(1)
	}
}
