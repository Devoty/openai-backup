package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg, usedFlags, err := parseFlags()
	if err != nil {
		exitWithError(err)
	}

	if err := loadPersistedConfig(cfg, usedFlags); err != nil {
		exitWithError(err)
	}
	applyEnvFallback(cfg, usedFlags)

	if err := runApp(cfg); err != nil {
		exitWithError(err)
	}
}

func runApp(cfg *cliConfig) error {
	logCloser, err := setupLogger(cfg.LogPath)
	if err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logCloser.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg.BaseURL = ensureBaseURL(cfg.BaseURL)
	cfg.ExportTarget = normalizeExportTarget(cfg.ExportTarget)
	cfg.Order = normalizeOrder(cfg.Order)
	cfg.PageSize = clampPageSize(cfg.PageSize)
	cfg.MaxConversations = nonNegative(cfg.MaxConversations)
	cfg.InitialOffset = nonNegative(cfg.InitialOffset)
	if strings.TrimSpace(cfg.UserAgent) == "" {
		cfg.UserAgent = defaultUserAgent
	}

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
	ServeAddr           string
}

func parseFlags() (*cliConfig, map[string]struct{}, error) {
	cfg := &cliConfig{}

	flag.StringVar(&cfg.ConfigDBPath, "config-db", defaultConfigDBPath, "配置持久化使用的 SQLite 文件路径")
	flag.StringVar(&cfg.ServeAddr, "listen", defaultListenAddr, "Web 界面监听地址")

	flag.StringVar(&cfg.BaseURL, "base-url", defaultBaseURL, "ChatGPT 接口基础地址")
	flag.StringVar(&cfg.ExportTarget, "target", exportTargetAnytype, "导出目标: anytype 或 notion")
	flag.StringVar(&cfg.Order, "order", defaultOrder, "对话排序: updated 或 created")
	flag.IntVar(&cfg.PageSize, "page-size", defaultPageSize, "每次拉取的对话数量, 1-100")
	flag.IntVar(&cfg.MaxConversations, "max", defaultMaxConversations, "最多导出多少条对话, 0 表示不限制")
	flag.IntVar(&cfg.InitialOffset, "offset", defaultInitialOffset, "从第几条开始拉取对话")
	flag.BoolVar(&cfg.IncludeArchived, "include-archived", false, "是否包含归档对话")
	flag.StringVar(&cfg.Token, "token", "", "OpenAI Bearer Token")

	flag.StringVar(&cfg.OutputTimezone, "timezone", "", "输出时区, 例如 UTC 或 Asia/Shanghai")
	flag.StringVar(&cfg.LogPath, "log-file", "", "日志文件路径")

	flag.Parse()

	usedFlags := make(map[string]struct{})
	flag.CommandLine.Visit(func(f *flag.Flag) {
		usedFlags[f.Name] = struct{}{}
	})

	cfg.ConfigDBPath = strings.TrimSpace(cfg.ConfigDBPath)
	if cfg.ConfigDBPath == "" {
		cfg.ConfigDBPath = defaultConfigDBPath
	}

	return cfg, usedFlags, nil
}

func exitWithError(err error) {
	logInfo("程序异常结束: %v", err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// loadPersistedConfig ensures the SQLite store exists, writes defaults when empty,
// and merges persisted values back into the CLI config without overriding explicit flags.
func loadPersistedConfig(cfg *cliConfig, usedFlags map[string]struct{}) error {
	if cfg == nil {
		return nil
	}

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

	applyPersistedString(usedFlags, "anytype-base-url", &cfg.AnytypeBaseURL, payload.AnytypeBaseURL)
	applyPersistedString(usedFlags, "anytype-version", &cfg.AnytypeVersion, payload.AnytypeVersion)
	applyPersistedString(usedFlags, "anytype-space-id", &cfg.AnytypeSpaceID, payload.AnytypeSpaceID)
	applyPersistedString(usedFlags, "anytype-type-key", &cfg.AnytypeTypeKey, payload.AnytypeTypeKey)
	applyPersistedString(usedFlags, "anytype-token", &cfg.AnytypeToken, payload.AnytypeToken)
	applyPersistedString(usedFlags, "notion-base-url", &cfg.NotionBaseURL, payload.NotionBaseURL)
	applyPersistedString(usedFlags, "notion-version", &cfg.NotionVersion, payload.NotionVersion)
	applyPersistedString(usedFlags, "notion-token", &cfg.NotionToken, payload.NotionToken)
	applyPersistedString(usedFlags, "notion-parent-type", &cfg.NotionParentType, payload.NotionParentType)
	applyPersistedString(usedFlags, "notion-parent-id", &cfg.NotionParentID, payload.NotionParentID)
	applyPersistedString(usedFlags, "notion-title-property", &cfg.NotionTitleProperty, payload.NotionTitleProperty)
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

func applyEnvFallback(cfg *cliConfig, usedFlags map[string]struct{}) {
	if cfg == nil {
		return
	}

	applyEnvString(usedFlags, "token", &cfg.Token, "CHATGPT_BEARER_TOKEN", "CHATGPT_TOKEN")
	applyEnvString(usedFlags, "base-url", &cfg.BaseURL, "CHATGPT_BASE_URL")
	applyEnvString(usedFlags, "user-agent", &cfg.UserAgent, "CHATGPT_USER_AGENT")

	applyEnvString(usedFlags, "timezone", &cfg.OutputTimezone, "CHATGPT_TIMEZONE")
	applyEnvString(usedFlags, "log-file", &cfg.LogPath, "CHATGPT_LOG_PATH")

	applyEnvString(usedFlags, "anytype-base-url", &cfg.AnytypeBaseURL, "ANYTYPE_BASE_URL")
	applyEnvString(usedFlags, "anytype-version", &cfg.AnytypeVersion, "ANYTYPE_VERSION")
	applyEnvString(usedFlags, "anytype-space-id", &cfg.AnytypeSpaceID, "ANYTYPE_SPACE_ID")
	applyEnvString(usedFlags, "anytype-type-key", &cfg.AnytypeTypeKey, "ANYTYPE_TYPE_KEY")
	applyEnvString(usedFlags, "anytype-token", &cfg.AnytypeToken, "ANYTYPE_TOKEN", "ANYTYPE_API_KEY")

	applyEnvString(usedFlags, "notion-base-url", &cfg.NotionBaseURL, "NOTION_BASE_URL")
	applyEnvString(usedFlags, "notion-version", &cfg.NotionVersion, "NOTION_VERSION")
	applyEnvString(usedFlags, "notion-token", &cfg.NotionToken, "NOTION_TOKEN", "NOTION_API_KEY")
	applyEnvString(usedFlags, "notion-parent-type", &cfg.NotionParentType, "NOTION_PARENT_TYPE")
	applyEnvString(usedFlags, "notion-parent-id", &cfg.NotionParentID, "NOTION_PARENT_ID")
	applyEnvString(usedFlags, "notion-title-property", &cfg.NotionTitleProperty, "NOTION_TITLE_PROPERTY")
}

func applyEnvString(usedFlags map[string]struct{}, flagName string, dst *string, envKeys ...string) {
	if dst == nil || flagUsed(usedFlags, flagName) {
		return
	}
	for _, key := range envKeys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			*dst = v
			return
		}
	}
}
