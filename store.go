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
	const configsSchema = `
CREATE TABLE IF NOT EXISTS configs (
	name TEXT PRIMARY KEY,
	payload BLOB NOT NULL,
	updated_at TIMESTAMP NOT NULL
);`
	if _, err := s.db.ExecContext(ctx, configsSchema); err != nil {
		return fmt.Errorf("初始化配置表失败: %w", err)
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
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	encrypted, err := s.encrypt(data)
	if err != nil {
		return fmt.Errorf("加密配置失败: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
INSERT INTO configs(name, payload, updated_at)
VALUES(?, ?, ?)
ON CONFLICT(name) DO UPDATE SET payload=excluded.payload, updated_at=excluded.updated_at
`, configStoreKey, encrypted, now)
	if err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
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
	var encrypted []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM configs WHERE name = ?`, configStoreKey).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return payload, errConfigNotFound
	}
	if err != nil {
		return payload, fmt.Errorf("读取配置失败: %w", err)
	}
	plain, err := s.decrypt(encrypted)
	if err != nil {
		return payload, fmt.Errorf("解密配置失败: %w", err)
	}
	if err := json.Unmarshal(plain, &payload); err != nil {
		return payload, fmt.Errorf("解析配置失败: %w", err)
	}
	return payload, nil
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
