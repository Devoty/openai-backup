package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// CLI 入口: 解析参数、初始化日志和 HTTP 客户端。
	cfg := parseFlags()
	target := strings.TrimSpace(cfg.ExportTarget)

	target = strings.ToLower(target)
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv(tokenEnvVar))
	}

	logCloser, err := setupLogger(cfg.LogPath)
	if err != nil {
		exitWithError(fmt.Errorf("初始化日志失败: %w", err))
	}
	defer logCloser.Close()

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if cfg.ServeMode {
		logInfo("启动 Web 界面, 输出时区=%s, 监听地址=%s, 默认导出目标=%s", cfg.OutputTimezone, cfg.ServeAddr, target)
		if err := runWebServer(ctx, client, cfg, token); err != nil {
			exitWithError(fmt.Errorf("启动 Web 界面失败: %w", err))
		}
		return
	}

	if target == "" {
		target = defaultExportTarget
	}

	if token == "" {
		exitWithError(errors.New("missing bearer token: provide --token or set " + tokenEnvVar))
	}
	logInfo("启动导出流程, 输出时区=%s, 目标=%s", cfg.OutputTimezone, target)

	conversations, err := fetchAllConversations(ctx, client, cfg, token)
	if err != nil {
		exitWithError(fmt.Errorf("获取对话列表失败: %w", err))
	}
	logInfo("对话列表获取完成, 数量=%d", len(conversations))

	var exports []exportConversation
	for _, meta := range conversations {
		logInfo("拉取对话详情: id=%s title=%s", meta.ID, meta.Title)
		detail, err := fetchConversationDetail(ctx, client, cfg, token, meta.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告: 获取对话详情失败, ID=%s, 原因=%v\n", meta.ID, err)
			logInfo("对话详情获取失败: id=%s err=%v", meta.ID, err)
			continue
		}
		export := buildExportConversation(meta, detail)
		if len(export.Messages) == 0 {
			logInfo("对话无有效消息, 跳过 id=%s", meta.ID)
			continue
		}
		exports = append(exports, export)
	}

	if len(exports) == 0 {
		exitWithError(errors.New("没有可导出的对话内容"))
	}

	switch target {
	case exportTargetAnytype:
		anyClient, err := newAnytypeClient(cfg, client)
		if err != nil {
			exitWithError(err)
		}

		created, err := syncConversationsToAnytype(ctx, anyClient, exports, cfg.OutputTimezone)
		if err != nil {
			exitWithError(err)
		}

		fmt.Printf("已导出 %d 个对话到 Anytype 空间 %s\n", created, cfg.AnytypeSpaceID)
		logInfo("导出完成, 对话数=%d, 目标=Anytype, Space=%s", created, cfg.AnytypeSpaceID)
	case exportTargetNotion:
		notionClient, err := newNotionClient(cfg, client)
		if err != nil {
			exitWithError(err)
		}

		created, pageIDs, err := syncConversationsToNotion(ctx, notionClient, exports, cfg.OutputTimezone)
		if err != nil {
			exitWithError(err)
		}

		fmt.Printf("已导出 %d 个对话到 Notion %s %s\n", created, firstNonEmpty(cfg.NotionParentType, "parent"), cfg.NotionParentID)
		if len(pageIDs) > 0 {
			logInfo("Notion 页面创建列表: %v", pageIDs)
		}
		logInfo("导出完成, 对话数=%d, 目标=Notion, ParentType=%s, ParentID=%s", created, cfg.NotionParentType, cfg.NotionParentID)
	default:
		exitWithError(fmt.Errorf("不支持的导出目标: %s", target))
	}
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
	DeviceID            string
	UserAgent           string
	AcceptLanguage      string
	Referer             string
	Cookie              string
	Origin              string
	OaiLanguage         string
	SecChUA             string
	SecChUAMobile       string
	SecChUAPlatform     string
	SecFetchDest        string
	SecFetchMode        string
	SecFetchSite        string
	ChatGPTAccountID    string
	OAIClientVersion    string
	Priority            string
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
	ConfigSecret        string
	ServeMode           bool
	ServeAddr           string
}

func parseFlags() *cliConfig {
	cfg := &cliConfig{}
	flag.StringVar(&cfg.Token, "token", "", "OpenAI Bearer Token (默认从环境变量 "+tokenEnvVar+" 读取)")
	flag.StringVar(&cfg.BaseURL, "base-url", defaultBaseURL, "接口基础地址")
	flag.StringVar(&cfg.OutputPath, "output", defaultOutputFile, "已废弃: 保留兼容性, 将忽略该参数")
	flag.StringVar(&cfg.Order, "order", "updated", "排序字段 (updated 或 created)")
	flag.IntVar(&cfg.PageSize, "page-size", 20, "每次请求的对话数量")
	flag.IntVar(&cfg.MaxConversations, "max", 0, "最多导出的对话数量 (0 表示全部)")
	flag.IntVar(&cfg.InitialOffset, "offset", 0, "从指定 offset 开始读取")
	flag.BoolVar(&cfg.IncludeArchived, "include-archived", false, "是否包含已归档对话")
	flag.StringVar(&cfg.OutputTimezone, "timezone", "Local", "输出内容中的时间时区 (Local 或 UTC)")
	flag.StringVar(&cfg.ExportTarget, "target", defaultExportTarget, "导出目标 (anytype 或 notion)")
	flag.StringVar(&cfg.DeviceID, "device-id", "", "oai-device-id 请求头 (默认从环境变量 "+deviceIDEnvVar+" 读取)")
	flag.StringVar(&cfg.UserAgent, "user-agent", "", "自定义 User-Agent (默认从环境变量 "+userAgentEnvVar+" 读取, 再回退内置值)")
	flag.StringVar(&cfg.AcceptLanguage, "accept-language", "", "Accept-Language 请求头 (默认从环境变量 "+acceptLangEnvVar+" 读取)")
	flag.StringVar(&cfg.Referer, "referer", "", "Referer 请求头 (默认从环境变量 "+refererEnvVar+" 读取)")
	flag.StringVar(&cfg.Cookie, "cookie", "", "Cookie 请求头 (默认从环境变量 "+cookieEnvVar+" 读取)")
	flag.StringVar(&cfg.Origin, "origin", "", "Origin 请求头 (默认从环境变量 "+originEnvVar+" 读取)")
	flag.StringVar(&cfg.OaiLanguage, "oai-language", "", "oai-language 请求头 (默认从环境变量 "+oaiLanguageEnvVar+" 读取)")
	flag.StringVar(&cfg.SecChUA, "sec-ch-ua", "", "sec-ch-ua 请求头 (默认从环境变量 "+secChUAEnvVar+" 读取)")
	flag.StringVar(&cfg.SecChUAMobile, "sec-ch-ua-mobile", "", "sec-ch-ua-mobile 请求头 (默认从环境变量 "+secChUAMobileEnv+" 读取)")
	flag.StringVar(&cfg.SecChUAPlatform, "sec-ch-ua-platform", "", "sec-ch-ua-platform 请求头 (默认从环境变量 "+secChUAPlatformEnv+" 读取)")
	flag.StringVar(&cfg.SecFetchDest, "sec-fetch-dest", "", "sec-fetch-dest 请求头 (默认从环境变量 "+secFetchDestEnv+" 读取)")
	flag.StringVar(&cfg.SecFetchMode, "sec-fetch-mode", "", "sec-fetch-mode 请求头 (默认从环境变量 "+secFetchModeEnv+" 读取)")
	flag.StringVar(&cfg.SecFetchSite, "sec-fetch-site", "", "sec-fetch-site 请求头 (默认从环境变量 "+secFetchSiteEnv+" 读取)")
	flag.StringVar(&cfg.ChatGPTAccountID, "chatgpt-account-id", "", "chatgpt-account-id 请求头 (默认从环境变量 "+accountIDEnvVar+" 读取)")
	flag.StringVar(&cfg.OAIClientVersion, "oai-client-version", "", "oai-client-version 请求头 (默认从环境变量 "+clientVersionEnvVar+" 读取)")
	flag.StringVar(&cfg.Priority, "priority", "", "priority 请求头 (默认从环境变量 "+priorityEnvVar+" 读取)")
	flag.StringVar(&cfg.LogPath, "log-file", "chatgpt_export.log", "日志输出文件路径")
	flag.StringVar(&cfg.AnytypeToken, "anytype-token", "", "Anytype API Key (默认从环境变量 "+anytypeTokenEnvVar+" 读取)")
	flag.StringVar(&cfg.AnytypeBaseURL, "anytype-base-url", "", "Anytype API 基础地址 (默认 "+defaultAnytypeBaseURL+")")
	flag.StringVar(&cfg.AnytypeVersion, "anytype-version", "", "Anytype API 版本 (默认 "+defaultAnytypeVersion+")")
	flag.StringVar(&cfg.AnytypeSpaceID, "anytype-space-id", "", "Anytype 目标空间 ID (默认从环境变量 "+anytypeSpaceIDEnvVar+" 读取)")
	flag.StringVar(&cfg.AnytypeTypeKey, "anytype-type-key", "", "Anytype 对象类型 key (默认 "+defaultAnytypeTypeKey+")")
	flag.StringVar(&cfg.NotionToken, "notion-token", "", "Notion API Key (默认从环境变量 "+notionTokenEnvVar+" 读取)")
	flag.StringVar(&cfg.NotionBaseURL, "notion-base-url", "", "Notion API 基础地址 (默认 "+defaultNotionBaseURL+")")
	flag.StringVar(&cfg.NotionVersion, "notion-version", "", "Notion API 版本 (默认 "+defaultNotionVersion+")")
	flag.StringVar(&cfg.NotionParentType, "notion-parent-type", "", "Notion 父级类型 (page 或 database)")
	flag.StringVar(&cfg.NotionParentID, "notion-parent-id", "", "Notion 父级页面/数据库 ID (默认从环境变量 "+notionParentIDEnvVar+" 读取)")
	flag.StringVar(&cfg.NotionTitleProperty, "notion-title-property", "", "Notion 标题属性名称 (数据库默认 "+defaultNotionDatabaseTitleProp+")")
	flag.StringVar(&cfg.ConfigDBPath, "config-db", defaultConfigDBPath, "配置持久化使用的 SQLite 文件路径")
	flag.StringVar(&cfg.ConfigSecret, "config-secret", "", "配置加密密钥 (默认从环境变量 "+configSecretEnvVar+" 读取)")
	flag.BoolVar(&cfg.ServeMode, "serve", false, "启动 Web 界面以浏览和导入对话")
	flag.StringVar(&cfg.ServeAddr, "listen", "127.0.0.1:8080", "Web 界面监听地址")
	flag.Parse()

	if cfg.UserAgent == "" {
		cfg.UserAgent = strings.TrimSpace(os.Getenv(userAgentEnvVar))
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.DeviceID == "" {
		cfg.DeviceID = strings.TrimSpace(os.Getenv(deviceIDEnvVar))
	}
	if cfg.AcceptLanguage == "" {
		cfg.AcceptLanguage = strings.TrimSpace(os.Getenv(acceptLangEnvVar))
	}
	if cfg.Referer == "" {
		cfg.Referer = strings.TrimSpace(os.Getenv(refererEnvVar))
	}
	if cfg.Cookie == "" {
		cfg.Cookie = strings.TrimSpace(os.Getenv(cookieEnvVar))
	}
	if cfg.Origin == "" {
		cfg.Origin = strings.TrimSpace(os.Getenv(originEnvVar))
	}
	if cfg.OaiLanguage == "" {
		cfg.OaiLanguage = strings.TrimSpace(os.Getenv(oaiLanguageEnvVar))
	}
	if cfg.SecChUA == "" {
		cfg.SecChUA = strings.TrimSpace(os.Getenv(secChUAEnvVar))
	}
	if cfg.SecChUAMobile == "" {
		cfg.SecChUAMobile = strings.TrimSpace(os.Getenv(secChUAMobileEnv))
	}
	if cfg.SecChUAPlatform == "" {
		cfg.SecChUAPlatform = strings.TrimSpace(os.Getenv(secChUAPlatformEnv))
	}
	if cfg.SecFetchDest == "" {
		cfg.SecFetchDest = strings.TrimSpace(os.Getenv(secFetchDestEnv))
	}
	if cfg.SecFetchMode == "" {
		cfg.SecFetchMode = strings.TrimSpace(os.Getenv(secFetchModeEnv))
	}
	if cfg.SecFetchSite == "" {
		cfg.SecFetchSite = strings.TrimSpace(os.Getenv(secFetchSiteEnv))
	}
	if cfg.ChatGPTAccountID == "" {
		cfg.ChatGPTAccountID = strings.TrimSpace(os.Getenv(accountIDEnvVar))
	}
	if cfg.OAIClientVersion == "" {
		cfg.OAIClientVersion = strings.TrimSpace(os.Getenv(clientVersionEnvVar))
	}
	if cfg.Priority == "" {
		cfg.Priority = strings.TrimSpace(os.Getenv(priorityEnvVar))
	}
	if cfg.AnytypeToken == "" {
		cfg.AnytypeToken = strings.TrimSpace(os.Getenv(anytypeTokenEnvVar))
	}
	if cfg.AnytypeBaseURL == "" {
		cfg.AnytypeBaseURL = strings.TrimSpace(os.Getenv(anytypeBaseURLEnvVar))
	}
	if cfg.AnytypeBaseURL == "" {
		cfg.AnytypeBaseURL = defaultAnytypeBaseURL
	} else {
		cfg.AnytypeBaseURL = strings.TrimSpace(cfg.AnytypeBaseURL)
	}
	if cfg.AnytypeVersion == "" {
		cfg.AnytypeVersion = strings.TrimSpace(os.Getenv(anytypeVersionEnvVar))
	}
	if cfg.AnytypeVersion == "" {
		cfg.AnytypeVersion = defaultAnytypeVersion
	}
	if cfg.AnytypeSpaceID == "" {
		cfg.AnytypeSpaceID = strings.TrimSpace(os.Getenv(anytypeSpaceIDEnvVar))
	}
	if cfg.AnytypeTypeKey == "" {
		cfg.AnytypeTypeKey = strings.TrimSpace(os.Getenv(anytypeTypeKeyEnvVar))
	}
	if cfg.AnytypeTypeKey == "" {
		cfg.AnytypeTypeKey = defaultAnytypeTypeKey
	}
	if cfg.ExportTarget == "" {
		cfg.ExportTarget = strings.TrimSpace(os.Getenv(exportTargetEnvVar))
	}
	cfg.ExportTarget = strings.ToLower(strings.TrimSpace(cfg.ExportTarget))
	if cfg.ExportTarget == "" {
		cfg.ExportTarget = defaultExportTarget
	}
	if cfg.NotionToken == "" {
		cfg.NotionToken = strings.TrimSpace(os.Getenv(notionTokenEnvVar))
	}
	if cfg.NotionBaseURL == "" {
		cfg.NotionBaseURL = strings.TrimSpace(os.Getenv(notionBaseURLEnvVar))
	}
	cfg.NotionBaseURL = strings.TrimSpace(cfg.NotionBaseURL)
	if cfg.NotionBaseURL == "" {
		cfg.NotionBaseURL = defaultNotionBaseURL
	} else {
		cfg.NotionBaseURL = strings.TrimRight(cfg.NotionBaseURL, "/")
	}
	if cfg.NotionVersion == "" {
		cfg.NotionVersion = strings.TrimSpace(os.Getenv(notionVersionEnvVar))
	}
	if cfg.NotionVersion == "" {
		cfg.NotionVersion = defaultNotionVersion
	}
	if cfg.NotionParentType == "" {
		cfg.NotionParentType = strings.TrimSpace(os.Getenv(notionParentTypeEnvVar))
	}
	cfg.NotionParentType = strings.ToLower(strings.TrimSpace(cfg.NotionParentType))
	if cfg.NotionParentID == "" {
		cfg.NotionParentID = strings.TrimSpace(os.Getenv(notionParentIDEnvVar))
	}
	if cfg.NotionTitleProperty == "" {
		cfg.NotionTitleProperty = strings.TrimSpace(os.Getenv(notionTitlePropertyEnvVar))
	}
	cfg.ConfigDBPath = strings.TrimSpace(cfg.ConfigDBPath)
	if cfg.ConfigDBPath == "" {
		cfg.ConfigDBPath = defaultConfigDBPath
	}
	if cfg.ConfigSecret == "" {
		cfg.ConfigSecret = strings.TrimSpace(os.Getenv(configSecretEnvVar))
	}
	cfg.ConfigSecret = strings.TrimSpace(cfg.ConfigSecret)
	return cfg
}

func exitWithError(err error) {
	logInfo("程序异常结束: %v", err)
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
