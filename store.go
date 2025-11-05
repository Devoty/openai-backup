package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
	_ "modernc.org/sqlite"
)

const (
	configStoreKey   = "active"
	metaKeySalt      = "key_salt"
	metaKeyHash      = "key_hash"
	metaTableName    = "metadata"
	minPasswordBytes = 8
)

var (
	errConfigNotFound   = errors.New("config not found")
	errStoreLocked      = errors.New("config store locked")
	errPasswordNotSet   = errors.New("password not set")
	errInvalidPassword  = errors.New("invalid password")
	errPasswordTooShort = errors.New("密码长度至少需要 8 个字符")
)

type configStore struct {
	db          *sql.DB
	key         []byte
	hasPassword bool
	unlocked    bool
}

func newConfigStore(path string) (*configStore, error) {
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

	store := &configStore{
		db: db,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.ensureSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	hasPassword, err := store.detectPassword(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}
	store.hasPassword = hasPassword
	store.unlocked = !hasPassword
	return store, nil
}

func (s *configStore) ensureSchema(ctx context.Context) error {
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

	const metaSchema = `
CREATE TABLE IF NOT EXISTS metadata (
	key TEXT PRIMARY KEY,
	value BLOB NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, metaSchema); err != nil {
		return fmt.Errorf("初始化配置元数据失败: %w", err)
	}
	return nil
}

func (s *configStore) detectPassword(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM metadata WHERE key = ? LIMIT 1`, metaKeySalt).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("检测配置密码失败: %w", err)
	}
	return true, nil
}

func (s *configStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *configStore) HasPassword() bool {
	if s == nil {
		return false
	}
	return s.hasPassword
}

func (s *configStore) Unlocked() bool {
	if s == nil {
		return false
	}
	return s.unlocked
}

func (s *configStore) SetPassword(ctx context.Context, password string) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if s.hasPassword {
		return errors.New("密码已存在，请使用修改密码接口")
	}
	password = strings.TrimSpace(password)
	if len(password) < minPasswordBytes {
		return errPasswordTooShort
	}
	salt := randomBytes(16)
	key, err := deriveKey(password, salt)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(key)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key, value) VALUES(?, ?)`, metaKeySalt, salt); err != nil {
		tx.Rollback()
		return fmt.Errorf("写入密码盐失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key, value) VALUES(?, ?)`, metaKeyHash, hash[:]); err != nil {
		tx.Rollback()
		return fmt.Errorf("写入密码校验失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.key = key
	s.hasPassword = true
	s.unlocked = true
	return nil
}

func (s *configStore) Unlock(ctx context.Context, password string) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if !s.hasPassword {
		return errPasswordNotSet
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return errInvalidPassword
	}

	salt, err := s.loadMetadataValue(ctx, metaKeySalt)
	if err != nil {
		return err
	}
	if len(salt) == 0 {
		return errors.New("配置存储缺少密码信息")
	}
	key, err := deriveKey(password, salt)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(key)
	existing, err := s.loadMetadataValue(ctx, metaKeyHash)
	if err != nil {
		return err
	}
	if !compareBytes(hash[:], existing) {
		return errInvalidPassword
	}
	s.key = key
	s.unlocked = true
	return nil
}

func (s *configStore) VerifyPassword(ctx context.Context, password string) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if !s.hasPassword {
		return errPasswordNotSet
	}
	password = strings.TrimSpace(password)
	if password == "" {
		return errInvalidPassword
	}

	salt, err := s.loadMetadataValue(ctx, metaKeySalt)
	if err != nil {
		return err
	}
	if len(salt) == 0 {
		return errors.New("配置存储缺少密码信息")
	}
	key, err := deriveKey(password, salt)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(key)
	existing, err := s.loadMetadataValue(ctx, metaKeyHash)
	if err != nil {
		return err
	}
	if !compareBytes(hash[:], existing) {
		return errInvalidPassword
	}
	return nil
}

func (s *configStore) UpdatePassword(ctx context.Context, newPassword string) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if !s.unlocked || s.key == nil {
		return errStoreLocked
	}
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < minPasswordBytes {
		return errPasswordTooShort
	}

	salt := randomBytes(16)
	key, err := deriveKey(newPassword, salt)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(key)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, metaKeySalt, salt); err != nil {
		tx.Rollback()
		return fmt.Errorf("更新密码盐失败: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, metaKeyHash, hash[:]); err != nil {
		tx.Rollback()
		return fmt.Errorf("更新密码校验失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.key = key
	s.hasPassword = true
	s.unlocked = true
	return nil
}

func (s *configStore) SaveConfig(ctx context.Context, payload configPayload) error {
	if s == nil {
		return errors.New("配置存储未初始化")
	}
	if !s.hasPassword {
		return errPasswordNotSet
	}
	if !s.unlocked || s.key == nil {
		return errStoreLocked
	}
	if err := s.persistConfigItems(ctx, payload); err != nil {
		return err
	}
	return nil
}

func (s *configStore) persistConfigItems(ctx context.Context, payload configPayload) error {
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
		if item.encrypted && item.value != "" {
			encrypted, encErr := s.encrypt(valueBytes)
			if encErr != nil {
				tx.Rollback()
				return fmt.Errorf("加密配置项 %s 失败: %w", key, encErr)
			}
			valueBytes = encrypted
			encryptedFlag = 1
		}
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

func (s *configStore) loadConfigItems(ctx context.Context) (configPayload, error) {
	var payload configPayload
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, encrypted FROM config_items`)
	if err != nil {
		return payload, fmt.Errorf("读取配置项失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			key          string
			value        []byte
			encryptedFlg int
		)
		if err := rows.Scan(&key, &value, &encryptedFlg); err != nil {
			return payload, fmt.Errorf("解析配置项失败: %w", err)
		}
		text := string(value)
		if encryptedFlg == 1 && len(value) > 0 {
			plain, decErr := s.decrypt(value)
			if decErr != nil {
				return payload, fmt.Errorf("解密配置项 %s 失败: %w", key, decErr)
			}
			text = string(plain)
		}
		applyConfigItem(&payload, key, text)
	}
	if err := rows.Err(); err != nil {
		return payload, fmt.Errorf("读取配置项失败: %w", err)
	}
	return normalizeConfigImportPayload(payload), nil
}

func (s *configStore) migrateLegacyConfig(ctx context.Context) error {
	if s == nil || s.db == nil || s.key == nil {
		return nil
	}
	var itemCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM config_items`).Scan(&itemCount); err != nil {
		return fmt.Errorf("统计配置项失败: %w", err)
	}
	if itemCount > 0 {
		return nil
	}
	var tableName string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='configs'`).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检测旧配置表失败: %w", err)
	}
	var encrypted []byte
	err = s.db.QueryRowContext(ctx, `SELECT payload FROM configs WHERE name = ?`, configStoreKey).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取旧配置失败: %w", err)
	}
	if len(encrypted) == 0 {
		return nil
	}
	plain, err := s.decrypt(encrypted)
	if err != nil {
		return fmt.Errorf("解密旧配置失败: %w", err)
	}
	var payload configPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return fmt.Errorf("解析旧配置失败: %w", err)
	}
	normalized := normalizeConfigImportPayload(payload)
	if err := s.persistConfigItems(ctx, normalized); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS configs`); err != nil {
		logInfo("删除旧配置表失败: %v", err)
	}
	return nil
}

func (s *configStore) LoadConfig(ctx context.Context) (configPayload, error) {
	var payload configPayload
	if s == nil {
		return payload, errConfigNotFound
	}
	if !s.hasPassword {
		return payload, errPasswordNotSet
	}
	if !s.unlocked || s.key == nil {
		return payload, errStoreLocked
	}
	if err := s.migrateLegacyConfig(ctx); err != nil {
		return payload, err
	}
	return s.loadConfigItems(ctx)
}

func (s *configStore) loadMetadataValue(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("metadata key %s not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("读取配置元数据失败: %w", err)
	}
	return value, nil
}

func (s *configStore) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (s *configStore) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	size := gcm.NonceSize()
	if len(data) < size {
		return nil, errors.New("密文长度异常")
	}
	nonce := data[:size]
	cipherText := data[size:]
	return gcm.Open(nil, nonce, cipherText, nil)
}

func deriveKey(password string, salt []byte) ([]byte, error) {
	const (
		N      = 1 << 15
		r      = 8
		p      = 1
		keyLen = 32
	)
	key, err := scrypt.Key([]byte(password), salt, N, r, p, keyLen)
	if err != nil {
		return nil, fmt.Errorf("派生加密密钥失败: %w", err)
	}
	return key, nil
}

func randomBytes(n int) []byte {
	buf := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, buf)
	return buf
}

func compareBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

type configItem struct {
	value     string
	encrypted bool
}

var sensitiveConfigKeys = map[string]struct{}{
	"token":         {},
	"cookie":        {},
	"anytype_token": {},
	"notion_token":  {},
}

func isSensitiveConfigKey(key string) bool {
	_, ok := sensitiveConfigKeys[key]
	return ok
}

func configPayloadToItems(payload configPayload) map[string]configItem {
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
		"token":                 {value: payload.Token, encrypted: true},
		"device_id":             {value: payload.DeviceID},
		"user_agent":            {value: payload.UserAgent},
		"accept_language":       {value: payload.AcceptLanguage},
		"referer":               {value: payload.Referer},
		"cookie":                {value: payload.Cookie, encrypted: true},
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
		"anytype_token":         {value: payload.AnytypeToken, encrypted: true},
		"notion_base_url":       {value: payload.NotionBaseURL},
		"notion_version":        {value: payload.NotionVersion},
		"notion_token":          {value: payload.NotionToken, encrypted: true},
		"notion_parent_type":    {value: payload.NotionParentType},
		"notion_parent_id":      {value: payload.NotionParentID},
		"notion_title_property": {value: payload.NotionTitleProperty},
	}
	for key, item := range items {
		if isSensitiveConfigKey(key) && item.value != "" {
			item.encrypted = true
			items[key] = item
		}
	}
	return items
}

func applyConfigItem(payload *configPayload, key, value string) {
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
