package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	errConfigNotFound = errors.New("config not found")
)

type ConfigStore struct {
	db *sql.DB
}

func Init(path string) (*ConfigStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("配置数据库路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout=5000", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开配置数据库失败: %w", err)
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &ConfigStore{
		db: db,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.ensureSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *ConfigStore) ensureSchema(ctx context.Context) error {
	const configItemsSchema = `
		CREATE TABLE IF NOT EXISTS config_items (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL,
			encrypted INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMP NOT NULL
		);`
	if _, err := s.db.ExecContext(ctx, configItemsSchema); err != nil {
		return fmt.Errorf("初始化配置项表失败: %w", err)
	}

	if err := s.ensureDefaultConfigItems(ctx); err != nil {
		return err
	}
	return nil
}

func (s *ConfigStore) ensureDefaultConfigItems(ctx context.Context) error {
	defaults := map[string]string{
		"listen":            defaultListenAddr,
		"timezone":          "",
		"target":            exportTargetAnytype,
		"base_url":          defaultBaseURL,
		"order":             defaultOrder,
		"page_size":         strconv.Itoa(defaultPageSize),
		"max_conversations": strconv.Itoa(defaultMaxConversations),
		"initial_offset":    strconv.Itoa(defaultInitialOffset),
		"include_archived":  strconv.FormatBool(false),
	}
	now := time.Now().UTC()
	for key, value := range defaults {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO config_items(key, value, encrypted, updated_at)
			VALUES(?, ?, 0, ?)
			ON CONFLICT(key) DO NOTHING
		`, key, []byte(value), now); err != nil {
			return fmt.Errorf("写入默认配置项 %s 失败: %w", key, err)
		}
	}
	return nil
}

func (s *ConfigStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// HasConfigItems reports whether at least one config entry exists.
func (s *ConfigStore) HasConfigItems(ctx context.Context) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("配置存储未初始化")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_items`).Scan(&count); err != nil {
		return false, fmt.Errorf("统计配置项失败: %w", err)
	}
	return count > 0, nil
}

// SaveConfig writes the normalized payload into SQLite。
func (s *ConfigStore) SaveConfig(ctx context.Context, payload ConfigPayload) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if err := s.persistConfigItems(ctx, payload); err != nil {
		return err
	}
	return nil
}

func (s *ConfigStore) persistConfigItems(ctx context.Context, payload ConfigPayload) error {
	items := configPayloadToItems(payload)
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	keys := make([]interface{}, 0, len(items))
	for key, item := range items {
		keys = append(keys, key)
		valueBytes := []byte(item.value)
		encryptedFlag := int64(0)
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO config_items(key, value, encrypted, updated_at)
				VALUES(?, ?, ?, ?)
				ON CONFLICT(key) DO UPDATE SET value=excluded.value, encrypted=excluded.encrypted, updated_at=excluded.updated_at
				`, key, valueBytes, encryptedFlag, now); err != nil {
			tx.Rollback()
			return fmt.Errorf("写入配置项 %s 失败: %w", key, err)
		}
	}
	if len(keys) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(keys)), ",")
		if _, err := tx.ExecContext(ctx, `DELETE FROM config_items WHERE key NOT IN (`+placeholders+`)`, keys...); err != nil {
			tx.Rollback()
			return fmt.Errorf("清理旧配置项失败: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM config_items`); err != nil {
			tx.Rollback()
			return fmt.Errorf("清理配置项失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *ConfigStore) loadConfigItems(ctx context.Context) (ConfigPayload, error) {
	var payload ConfigPayload
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM config_items`)
	if err != nil {
		return payload, fmt.Errorf("读取配置项失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key   string
			value []byte
		)
		if err := rows.Scan(&key, &value); err != nil {
			return payload, fmt.Errorf("解析配置项失败: %w", err)
		}
		text := string(value)
		applyConfigItem(&payload, key, text)
	}
	if err := rows.Err(); err != nil {
		return payload, fmt.Errorf("读取配置项失败: %w", err)
	}
	return normalizeConfigImportPayload(payload), nil
}

// LoadConfig 读取并返回归一化后的配置。
func (s *ConfigStore) LoadConfig(ctx context.Context) (ConfigPayload, error) {
	var payload ConfigPayload
	if s == nil {
		return payload, errConfigNotFound
	}
	hasConfig, err := s.HasConfigItems(ctx)
	if err != nil {
		return payload, err
	}
	if !hasConfig {
		return payload, errConfigNotFound
	}
	return s.loadConfigItems(ctx)
}

type configItem struct {
	value string
}

func configPayloadToItems(payload ConfigPayload) map[string]configItem {
	items := map[string]configItem{
		"listen":                {value: payload.Listen},
		"timezone":              {value: payload.Timezone},
		"target":                {value: payload.Target},
		"base_url":              {value: payload.BaseURL},
		"order":                 {value: payload.Order},
		"page_size":             {value: strconv.Itoa(payload.PageSize)},
		"max_conversations":     {value: strconv.Itoa(payload.MaxConversations)},
		"initial_offset":        {value: strconv.Itoa(payload.InitialOffset)},
		"include_archived":      {value: strconv.FormatBool(payload.IncludeArchived)},
		"token":                 {value: payload.Token},
		"device_id":             {value: payload.DeviceID},
		"user_agent":            {value: payload.UserAgent},
		"accept_language":       {value: payload.AcceptLanguage},
		"referer":               {value: payload.Referer},
		"cookie":                {value: payload.Cookie},
		"origin":                {value: payload.Origin},
		"oai_language":          {value: payload.OaiLanguage},
		"sec_ch_ua":             {value: payload.SecChUA},
		"sec_ch_ua_mobile":      {value: payload.SecChUAMobile},
		"sec_ch_ua_platform":    {value: payload.SecChUAPlatform},
		"sec_fetch_dest":        {value: payload.SecFetchDest},
		"sec_fetch_mode":        {value: payload.SecFetchMode},
		"sec_fetch_site":        {value: payload.SecFetchSite},
		"chatgpt_account_id":    {value: payload.ChatGPTAccountID},
		"oai_client_version":    {value: payload.OAIClientVersion},
		"priority":              {value: payload.Priority},
		"log_path":              {value: payload.LogPath},
		"anytype_base_url":      {value: payload.AnytypeBaseURL},
		"anytype_version":       {value: payload.AnytypeVersion},
		"anytype_space_id":      {value: payload.AnytypeSpaceID},
		"anytype_type_key":      {value: payload.AnytypeTypeKey},
		"anytype_token":         {value: payload.AnytypeToken},
		"notion_base_url":       {value: payload.NotionBaseURL},
		"notion_version":        {value: payload.NotionVersion},
		"notion_token":          {value: payload.NotionToken},
		"notion_parent_type":    {value: payload.NotionParentType},
		"notion_parent_id":      {value: payload.NotionParentID},
		"notion_title_property": {value: payload.NotionTitleProperty},
	}
	return items
}

func applyConfigItem(payload *ConfigPayload, key, value string) {
	if payload == nil {
		return
	}
	switch key {
	case "listen":
		payload.Listen = strings.TrimSpace(value)
	case "timezone":
		payload.Timezone = strings.TrimSpace(value)
	case "target":
		payload.Target = strings.TrimSpace(value)
	case "base_url":
		payload.BaseURL = strings.TrimSpace(value)
	case "order":
		payload.Order = strings.TrimSpace(value)
	case "page_size":
		if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			payload.PageSize = v
		}
	case "max_conversations":
		if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			payload.MaxConversations = v
		}
	case "initial_offset":
		if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			payload.InitialOffset = v
		}
	case "include_archived":
		if b, err := strconv.ParseBool(strings.TrimSpace(value)); err == nil {
			payload.IncludeArchived = b
		}
	case "token":
		payload.Token = strings.TrimSpace(value)
	case "device_id":
		payload.DeviceID = strings.TrimSpace(value)
	case "user_agent":
		payload.UserAgent = strings.TrimSpace(value)
	case "accept_language":
		payload.AcceptLanguage = strings.TrimSpace(value)
	case "referer":
		payload.Referer = strings.TrimSpace(value)
	case "cookie":
		payload.Cookie = strings.TrimSpace(value)
	case "origin":
		payload.Origin = strings.TrimSpace(value)
	case "oai_language":
		payload.OaiLanguage = strings.TrimSpace(value)
	case "sec_ch_ua":
		payload.SecChUA = strings.TrimSpace(value)
	case "sec_ch_ua_mobile":
		payload.SecChUAMobile = strings.TrimSpace(value)
	case "sec_ch_ua_platform":
		payload.SecChUAPlatform = strings.TrimSpace(value)
	case "sec_fetch_dest":
		payload.SecFetchDest = strings.TrimSpace(value)
	case "sec_fetch_mode":
		payload.SecFetchMode = strings.TrimSpace(value)
	case "sec_fetch_site":
		payload.SecFetchSite = strings.TrimSpace(value)
	case "chatgpt_account_id":
		payload.ChatGPTAccountID = strings.TrimSpace(value)
	case "oai_client_version":
		payload.OAIClientVersion = strings.TrimSpace(value)
	case "priority":
		payload.Priority = strings.TrimSpace(value)
	case "log_path":
		payload.LogPath = strings.TrimSpace(value)
	case "anytype_base_url":
		payload.AnytypeBaseURL = strings.TrimSpace(value)
	case "anytype_version":
		payload.AnytypeVersion = strings.TrimSpace(value)
	case "anytype_space_id":
		payload.AnytypeSpaceID = strings.TrimSpace(value)
	case "anytype_type_key":
		payload.AnytypeTypeKey = strings.TrimSpace(value)
	case "anytype_token":
		payload.AnytypeToken = strings.TrimSpace(value)
	case "notion_base_url":
		payload.NotionBaseURL = strings.TrimSpace(value)
	case "notion_version":
		payload.NotionVersion = strings.TrimSpace(value)
	case "notion_token":
		payload.NotionToken = strings.TrimSpace(value)
	case "notion_parent_type":
		payload.NotionParentType = strings.TrimSpace(value)
	case "notion_parent_id":
		payload.NotionParentID = strings.TrimSpace(value)
	case "notion_title_property":
		payload.NotionTitleProperty = strings.TrimSpace(value)
	}
}
