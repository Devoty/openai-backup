package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// 解析命令行参数
	cfg, err := parseFlags()
	if err != nil {
		exitWithError(err)
	}

	// 加载持久化配置并合并
	if err := loadPersistedConfig(cfg); err != nil {
		exitWithError(err)
	}

	// 启动应用主逻辑
	if err := runApp(cfg); err != nil {
		exitWithError(err)
	}
}

func runApp(cfg *cliConfig) error {
	logCloser, err := setupLogger(cfg.LogPath)
	if err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer func(logCloser io.Closer) {
		err := logCloser.Close()
		if err != nil {

		}
	}(logCloser)

	//创建一个 context，当程序收到指定信号（如 Ctrl+C 或 SIGTERM）时自动取消。用于优雅退出。
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logInfo("启动 Web 界面, 输出时区=%s, 监听地址=%s", cfg.OutputTimezone, cfg.ServeAddr)
	if err := runWebServer(ctx, cfg); err != nil {
		return fmt.Errorf("启动 Web 界面失败: %w", err)
	}
	return nil

}

type cliConfig struct {
	BaseURL             string
	OutputPath          string
	Order               string
	PageSize            int
	MaxConversations    int
	InitialOffset       int
	IncludeArchived     bool
	Token               string
	OutputTimezone      string
	UserAgent           string
	LogPath             string
	AnytypeBaseURL      string
	AnytypeVersion      string
	AnytypeSpaceID      string
	AnytypeTypeKey      string
	AnytypeToken        string
	NotionBaseURL       string
	NotionVersion       string
	NotionToken         string
	NotionParentType    string
	NotionParentID      string
	NotionTitleProperty string
	ExportTarget        string
	ConfigDBPath        string
	ServeMode           bool
	ServeAddr           string
}

func parseFlags() (*cliConfig, error) {
	cfg := &cliConfig{}
	flag.StringVar(&cfg.ConfigDBPath, "config-db", defaultConfigDBPath, "配置持久化使用的 SQLite 文件路径")
	flag.StringVar(&cfg.ServeAddr, "listen", "127.0.0.1:8080", "Web 界面监听地址")
	flag.Parse()

	cfg.ConfigDBPath = strings.TrimSpace(cfg.ConfigDBPath)
	if cfg.ConfigDBPath == "" {
		cfg.ConfigDBPath = defaultConfigDBPath
	}

	return cfg, nil
}

func exitWithError(err error) {
	logInfo("程序异常结束: %v", err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// loadPersistedConfig ensures the SQLite store exists, writes defaults when empty,
// and merges persisted values back into the CLI config without overriding explicit flags.
func loadPersistedConfig(cfg *cliConfig) error {
	if cfg == nil {
		return nil
	}

	usedFlags := make(map[string]struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := Init(cfg.ConfigDBPath)
	if err != nil {
		return fmt.Errorf("初始化配置存储失败: %w", err)
	}
	defer store.Close()

	hasConfig, err := store.HasConfigItems(ctx)
	if err != nil {
		return fmt.Errorf("检查配置状态失败: %w", err)
	}
	if !hasConfig {
		payload := configToPayload(cfg)
		if err := store.SaveConfig(ctx, payload); err != nil {
			return fmt.Errorf("写入默认配置失败: %w", err)
		}
		applyPersistedConfig(cfg, payload, usedFlags)
		return nil
	}

	payload, err := store.LoadConfig(ctx)
	if err != nil {
		if errors.Is(err, errConfigNotFound) {
			return nil
		}
		return fmt.Errorf("读取配置失败: %w", err)
	}
	applyPersistedConfig(cfg, payload, usedFlags)
	return nil
}

func applyPersistedConfig(cfg *cliConfig, payload ConfigPayload, usedFlags map[string]struct{}) {
	if cfg == nil {
		return
	}
	applyPersistedString(usedFlags, "listen", &cfg.ServeAddr, payload.Listen)
	applyPersistedString(usedFlags, "timezone", &cfg.OutputTimezone, payload.Timezone)
	if !flagUsed(usedFlags, "target") {
		cfg.ExportTarget = normalizeExportTarget(payload.Target)
	}
	if !flagUsed(usedFlags, "base-url") {
		cfg.BaseURL = ensureBaseURL(payload.BaseURL)
	}
	if !flagUsed(usedFlags, "order") {
		cfg.Order = normalizeOrder(payload.Order)
	}
	applyPersistedInt(usedFlags, "page-size", &cfg.PageSize, payload.PageSize)
	applyPersistedInt(usedFlags, "max", &cfg.MaxConversations, payload.MaxConversations)
	applyPersistedInt(usedFlags, "offset", &cfg.InitialOffset, payload.InitialOffset)
	applyPersistedBool(usedFlags, "include-archived", &cfg.IncludeArchived, payload.IncludeArchived)
	applyPersistedString(usedFlags, "token", &cfg.Token, payload.Token)
	applyPersistedString(usedFlags, "user-agent", &cfg.UserAgent, payload.UserAgent)
	applyPersistedString(usedFlags, "log-file", &cfg.LogPath, payload.LogPath)
}

func applyPersistedString(usedFlags map[string]struct{}, flagName string, dst *string, value string) {
	if dst == nil || flagUsed(usedFlags, flagName) {
		return
	}
	*dst = strings.TrimSpace(value)
}

func applyPersistedInt(usedFlags map[string]struct{}, flagName string, dst *int, value int) {
	if dst == nil || flagUsed(usedFlags, flagName) {
		return
	}
	*dst = value
}

func applyPersistedBool(usedFlags map[string]struct{}, flagName string, dst *bool, value bool) {
	if dst == nil || flagUsed(usedFlags, flagName) {
		return
	}
	*dst = value
}

func flagUsed(usedFlags map[string]struct{}, name string) bool {
	if name == "" || usedFlags == nil {
		return false
	}
	_, ok := usedFlags[name]
	return ok
}
