package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRawDump(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dumps-test-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dm := NewDumpManager(tempDir)
	reqID := "test-req-999"

	rawReq := []byte("POST /v1/chat/completions HTTP/1.1\nHost: codegate\n\n{\"model\":\"test\"}")
	convertedReq := []byte("POST /chat/completions HTTP/1.1\nHost: upstream\n\n{\"model\":\"test\"}")
	backendResp := []byte("HTTP/1.1 500 Internal Server Error\n\n{\"error\":\"upstream crash\"}")
	clientResp := []byte("HTTP/1.1 502 Bad Gateway\n\n{\"error\":\"bad gateway\"}")

	err = dm.SaveRawDump(reqID, 500, rawReq, convertedReq, backendResp, clientResp)
	if err != nil {
		t.Fatalf("SaveRawDump 失败: %v", err)
	}

	dumpSubDir := filepath.Join(tempDir, reqID)
	files := []string{
		"1_raw_request.txt",
		"2_converted_request.txt",
		"3_500_backend_response.txt",
		"4_500_converted_response.txt",
	}

	for _, f := range files {
		p := filepath.Join(dumpSubDir, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("4阶段转储文件未生成: %s", p)
		}
	}
}
