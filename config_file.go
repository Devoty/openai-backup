package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = "config.json"

type fileConfig struct {
	BaseURL             *string `json:"base_url"`
	OutputPath          *string `json:"output_path"`
	Order               *string `json:"order"`
	PageSize            *int    `json:"page_size"`
	MaxConversations    *int    `json:"max_conversations"`
	InitialOffset       *int    `json:"initial_offset"`
	IncludeArchived     *bool   `json:"include_archived"`
	Token               *string `json:"token"`
	OutputTimezone      *string `json:"output_timezone"`
	DeviceID            *string `json:"device_id"`
	UserAgent           *string `json:"user_agent"`
	AcceptLanguage      *string `json:"accept_language"`
	Referer             *string `json:"referer"`
	Cookie              *string `json:"cookie"`
	Origin              *string `json:"origin"`
	OaiLanguage         *string `json:"oai_language"`
	SecChUA             *string `json:"sec_ch_ua"`
	SecChUAMobile       *string `json:"sec_ch_ua_mobile"`
	SecChUAPlatform     *string `json:"sec_ch_ua_platform"`
	SecFetchDest        *string `json:"sec_fetch_dest"`
	SecFetchMode        *string `json:"sec_fetch_mode"`
	SecFetchSite        *string `json:"sec_fetch_site"`
	ChatGPTAccountID    *string `json:"chatgpt_account_id"`
	OAIClientVersion    *string `json:"oai_client_version"`
	Priority            *string `json:"priority"`
	LogPath             *string `json:"log_path"`
	AnytypeBaseURL      *string `json:"anytype_base_url"`
	AnytypeVersion      *string `json:"anytype_version"`
	AnytypeSpaceID      *string `json:"anytype_space_id"`
	AnytypeTypeKey      *string `json:"anytype_type_key"`
	AnytypeToken        *string `json:"anytype_token"`
	NotionBaseURL       *string `json:"notion_base_url"`
	NotionVersion       *string `json:"notion_version"`
	NotionToken         *string `json:"notion_token"`
	NotionParentType    *string `json:"notion_parent_type"`
	NotionParentID      *string `json:"notion_parent_id"`
	NotionTitleProperty *string `json:"notion_title_property"`
	ExportTarget        *string `json:"export_target"`
	ConfigDBPath        *string `json:"config_db_path"`
	ConfigSecret        *string `json:"config_secret"`
	ServeMode           *bool   `json:"serve_mode"`
	ServeAddr           *string `json:"serve_addr"`
}

func defaultConfigFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return filepath.Join(".", configFileName)
	}
	return filepath.Join(configDir, "openai-backup", configFileName)
}

func resolveConfigFilePath(input string) (string, error) {
	path := strings.TrimSpace(input)
	if path == "" {
		return defaultConfigFilePath(), nil
	}

	path = expandUserHome(path)

	info, err := os.Stat(path)
	var resolved string
	switch {
	case err == nil && info.IsDir():
		resolved = filepath.Join(path, configFileName)
	case err == nil:
		resolved = path
	case errors.Is(err, fs.ErrNotExist):
		if strings.HasSuffix(path, string(os.PathSeparator)) || filepath.Ext(path) == "" {
			resolved = filepath.Join(path, configFileName)
		} else {
			resolved = path
		}
	default:
		return "", err
	}

	absPath, absErr := filepath.Abs(resolved)
	if absErr != nil {
		return resolved, nil
	}
	return absPath, nil
}

func loadFileConfig(path string) (*fileConfig, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("配置文件路径指向目录: " + path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &fileConfig{}, nil
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyFileConfig(cfg *cliConfig, fc *fileConfig, used map[string]struct{}) {
	if fc == nil {
		return
	}

	applyString(used, "base-url", &cfg.BaseURL, fc.BaseURL)
	applyString(used, "output", &cfg.OutputPath, fc.OutputPath)
	applyString(used, "order", &cfg.Order, fc.Order)
	applyInt(used, "page-size", &cfg.PageSize, fc.PageSize)
	applyInt(used, "max", &cfg.MaxConversations, fc.MaxConversations)
	applyInt(used, "offset", &cfg.InitialOffset, fc.InitialOffset)
	applyBool(used, "include-archived", &cfg.IncludeArchived, fc.IncludeArchived)
	applyString(used, "token", &cfg.Token, fc.Token)
	applyString(used, "timezone", &cfg.OutputTimezone, fc.OutputTimezone)
	applyString(used, "device-id", &cfg.DeviceID, fc.DeviceID)
	applyString(used, "user-agent", &cfg.UserAgent, fc.UserAgent)
	applyString(used, "accept-language", &cfg.AcceptLanguage, fc.AcceptLanguage)
	applyString(used, "referer", &cfg.Referer, fc.Referer)
	applyString(used, "cookie", &cfg.Cookie, fc.Cookie)
	applyString(used, "origin", &cfg.Origin, fc.Origin)
	applyString(used, "oai-language", &cfg.OaiLanguage, fc.OaiLanguage)
	applyString(used, "sec-ch-ua", &cfg.SecChUA, fc.SecChUA)
	applyString(used, "sec-ch-ua-mobile", &cfg.SecChUAMobile, fc.SecChUAMobile)
	applyString(used, "sec-ch-ua-platform", &cfg.SecChUAPlatform, fc.SecChUAPlatform)
	applyString(used, "sec-fetch-dest", &cfg.SecFetchDest, fc.SecFetchDest)
	applyString(used, "sec-fetch-mode", &cfg.SecFetchMode, fc.SecFetchMode)
	applyString(used, "sec-fetch-site", &cfg.SecFetchSite, fc.SecFetchSite)
	applyString(used, "chatgpt-account-id", &cfg.ChatGPTAccountID, fc.ChatGPTAccountID)
	applyString(used, "oai-client-version", &cfg.OAIClientVersion, fc.OAIClientVersion)
	applyString(used, "priority", &cfg.Priority, fc.Priority)
	applyString(used, "log-file", &cfg.LogPath, fc.LogPath)
	applyString(used, "anytype-base-url", &cfg.AnytypeBaseURL, fc.AnytypeBaseURL)
	applyString(used, "anytype-version", &cfg.AnytypeVersion, fc.AnytypeVersion)
	applyString(used, "anytype-space-id", &cfg.AnytypeSpaceID, fc.AnytypeSpaceID)
	applyString(used, "anytype-type-key", &cfg.AnytypeTypeKey, fc.AnytypeTypeKey)
	applyString(used, "anytype-token", &cfg.AnytypeToken, fc.AnytypeToken)
	applyString(used, "notion-base-url", &cfg.NotionBaseURL, fc.NotionBaseURL)
	applyString(used, "notion-version", &cfg.NotionVersion, fc.NotionVersion)
	applyString(used, "notion-token", &cfg.NotionToken, fc.NotionToken)
	applyString(used, "notion-parent-type", &cfg.NotionParentType, fc.NotionParentType)
	applyString(used, "notion-parent-id", &cfg.NotionParentID, fc.NotionParentID)
	applyString(used, "notion-title-property", &cfg.NotionTitleProperty, fc.NotionTitleProperty)
	applyString(used, "target", &cfg.ExportTarget, fc.ExportTarget)
	applyString(used, "config-db", &cfg.ConfigDBPath, fc.ConfigDBPath)
	applyString(used, "config-secret", &cfg.ConfigSecret, fc.ConfigSecret)
	applyBool(used, "serve", &cfg.ServeMode, fc.ServeMode)
	applyString(used, "listen", &cfg.ServeAddr, fc.ServeAddr)
}

func applyString(used map[string]struct{}, flagName string, dst *string, value *string) {
	if value == nil {
		return
	}
	if flagName != "" {
		if _, ok := used[flagName]; ok {
			return
		}
	}
	*dst = strings.TrimSpace(*value)
}

func applyInt(used map[string]struct{}, flagName string, dst *int, value *int) {
	if value == nil {
		return
	}
	if flagName != "" {
		if _, ok := used[flagName]; ok {
			return
		}
	}
	*dst = *value
}

func applyBool(used map[string]struct{}, flagName string, dst *bool, value *bool) {
	if value == nil {
		return
	}
	if flagName != "" {
		if _, ok := used[flagName]; ok {
			return
		}
	}
	*dst = *value
}

func expandUserHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(home, path[2:])
	}
	return path
}
