package main

import (
	"io"
	"log"
	"os"
	"strings"
)

var (
	nullLogger = log.New(io.Discard, "", log.LstdFlags)
	logger     = nullLogger
)

func setupLogger(path string) (io.Closer, error) {
	// 初始化日志: 同时写入文件和 stderr, 方便排查问题。
	if strings.TrimSpace(path) == "" {
		path = "chatgpt_export.log"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	multi := io.MultiWriter(file, os.Stderr)
	logger = log.New(multi, "", log.LstdFlags)
	logInfo("日志初始化完成, 输出文件=%s", path)
	return file, nil
}

func logInfo(format string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Printf(format, args...)
}
