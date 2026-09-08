package audit

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"code-gate/internal/models"
	"gorm.io/gorm"
)

// DumpManager 4 阶段原始报文转储管理器
type DumpManager struct {
	baseDir string
}

// NewDumpManager 创建转储管理器
func NewDumpManager(baseDir string) *DumpManager {
	if baseDir == "" {
		baseDir = "logs/dumps"
	}
	_ = os.MkdirAll(baseDir, 0755)
	return &DumpManager{baseDir: baseDir}
}

// SaveRawDump 在发生异常或调试诊断时记录 4 阶段原始报文
func (dm *DumpManager) SaveRawDump(
	reqID string,
	statusCode int,
	rawReq []byte,
	convertedReq []byte,
	backendResp []byte,
	clientResp []byte,
) error {
	if reqID == "" {
		reqID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	dumpDir := filepath.Join(dm.baseDir, reqID)
	if err := os.MkdirAll(dumpDir, 0755); err != nil {
		return err
	}

	// 1. 客户端原始请求
	if len(rawReq) > 0 {
		_ = os.WriteFile(filepath.Join(dumpDir, "1_raw_request.txt"), rawReq, 0644)
	}

	// 2. 发往后端的转换后请求
	if len(convertedReq) > 0 {
		_ = os.WriteFile(filepath.Join(dumpDir, "2_converted_request.txt"), convertedReq, 0644)
	}

	// 3. 后端原始响应
	if len(backendResp) > 0 {
		respName := fmt.Sprintf("3_%d_backend_response.txt", statusCode)
		_ = os.WriteFile(filepath.Join(dumpDir, respName), backendResp, 0644)
	}

	// 4. 网关最终回传给客户端的报文
	if len(clientResp) > 0 {
		clientRespName := fmt.Sprintf("4_%d_converted_response.txt", statusCode)
		_ = os.WriteFile(filepath.Join(dumpDir, clientRespName), clientResp, 0644)
	}

	log.Printf("[Audit] 已转储请求 [%s] 4 阶段原始排障报文至: %s", reqID, dumpDir)
	return nil
}

// StartLogCleanupTask 启动定期审计日志轮转清理后台协程
func StartLogCleanupTask(db *gorm.DB, retentionDays int, interval time.Duration) chan struct{} {
	if retentionDays <= 0 {
		retentionDays = 7 // 默认保留 7 天
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	stopChan := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		cleanOnce := func() {
			cutoff := time.Now().AddDate(0, 0, -retentionDays)
			res := db.Where("created_at < ?", cutoff).Delete(&models.AccessLog{})
			if res.Error == nil && res.RowsAffected > 0 {
				log.Printf("[Audit] 历史日志自动清理完成: 已清理 %d 条超过 %d 天的访问审计记录",
					res.RowsAffected, retentionDays)
			}
		}

		// 启动执行一次
		cleanOnce()

		for {
			select {
			case <-ticker.C:
				cleanOnce()
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan
}
