package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"code-gate/internal/models"
	"github.com/gin-gonic/gin"
)

// ProxyResult 代理转发完成后的元数据
type ProxyResult struct {
	StatusCode int
	Usage      TokenUsage
	DurationMS int64
	TTFTMS     int64
	Error      error
}

// Client 代理转发客户端
type Client struct {
	httpClient *http.Client
}

// NewClient 创建代理客户端
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // 防止破坏流式 SSE
	}
	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// ForwardChatCompletions 转发 OpenAI Chat 请求（兼容流式与非流式）
func (c *Client) ForwardChatCompletions(
	ctx *gin.Context,
	backend *models.Backend,
	bodyBytes []byte,
	isStream bool,
) (*ProxyResult, error) {
	startTime := time.Now()
	res := &ProxyResult{}

	// 目标 URL 拼装
	targetURL := strings.TrimRight(backend.BaseURL, "/") + "/chat/completions"
	if !strings.HasSuffix(backend.BaseURL, "/v1") && !strings.Contains(backend.BaseURL, "/v1/") {
		targetURL = strings.TrimRight(backend.BaseURL, "/") + "/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		res.Error = err
		return res, fmt.Errorf("创建上游请求失败: %w", err)
	}

	// 复制与设置请求头
	req.Header.Set("Content-Type", "application/json")
	if backend.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+backend.APIKey)
	}
	// 透传会话特征
	for _, h := range []string{"X-Session-ID", "X-Conversation-ID", "Session-ID", "Conversation-ID"} {
		if val := ctx.GetHeader(h); val != "" {
			req.Header.Set(h, val)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		res.Error = err
		res.DurationMS = time.Since(startTime).Milliseconds()
		return res, fmt.Errorf("上游模型实例通信失败 (%s): %w", backend.Name, err)
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode

	if isStream {
		return c.handleStreamResponse(ctx, resp, startTime)
	}
	return c.handleStandardResponse(ctx, resp, startTime)
}

// handleStreamResponse 处理 SSE 流式响应
func (c *Client) handleStreamResponse(ctx *gin.Context, resp *http.Response, startTime time.Time) (*ProxyResult, error) {
	res := &ProxyResult{StatusCode: resp.StatusCode}

	// 设置 SSE 响应标头
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("Transfer-Encoding", "chunked")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")
	ctx.Writer.WriteHeader(resp.StatusCode)
	ctx.Writer.Flush()

	// 启动后台 Keep-Alive Ping 协程，防止网关前置中间件因大模型长耗时思考而切断连接
	pingCtx, pingCancel := context.WithCancel(ctx.Request.Context())
	defer pingCancel()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = ctx.Writer.Write([]byte(": ping\n\n"))
				ctx.Writer.Flush()
			case <-pingCtx.Done():
				return
			}
		}
	}()

	reader := bufio.NewReader(resp.Body)
	var ttftRecorded bool
	var finalUsage TokenUsage

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if !ttftRecorded {
				res.TTFTMS = time.Since(startTime).Milliseconds()
				ttftRecorded = true
			}

			// 写回客户端
			_, writeErr := ctx.Writer.Write(line)
			if writeErr != nil {
				res.Error = writeErr
				break
			}
			ctx.Writer.Flush()

			// 检查解析 Usage 信息
			lineStr := strings.TrimSpace(string(line))
			if strings.HasPrefix(lineStr, "data: ") {
				payload := strings.TrimPrefix(lineStr, "data: ")
				if payload != "[DONE]" {
					if u := ParseUsageFromChunk([]byte(payload)); u != nil {
						finalUsage = *u
					}
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				res.Error = err
			}
			break
		}
	}

	res.Usage = finalUsage
	res.DurationMS = time.Since(startTime).Milliseconds()
	return res, nil
}

// handleStandardResponse 处理非流式响应
func (c *Client) handleStandardResponse(ctx *gin.Context, resp *http.Response, startTime time.Time) (*ProxyResult, error) {
	res := &ProxyResult{StatusCode: resp.StatusCode}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = err
		res.DurationMS = time.Since(startTime).Milliseconds()
		return res, fmt.Errorf("读取上游响应体失败: %w", err)
	}

	res.DurationMS = time.Since(startTime).Milliseconds()
	res.TTFTMS = res.DurationMS

	// 提取 Usage
	if u := ParseUsageFromChunk(body); u != nil {
		res.Usage = *u
	}

	// 复制上游标头并输出
	for k, v := range resp.Header {
		for _, vv := range v {
			ctx.Writer.Header().Add(k, vv)
		}
	}
	ctx.Writer.WriteHeader(resp.StatusCode)
	_, _ = ctx.Writer.Write(body)

	return res, nil
}
