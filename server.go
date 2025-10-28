package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	conversationCacheTTL = 30 * time.Second
	detailCacheTTL       = 5 * time.Minute
)

type detailCacheEntry struct {
	export  exportConversation
	fetched time.Time
}

type conversationPageCacheEntry struct {
	data    *conversationListResponse
	fetched time.Time
}

type convPageKey struct {
	offset int
	limit  int
}

func cloneConversationPage(src *conversationListResponse) *conversationListResponse {
	if src == nil {
		return nil
	}
	copy := *src
	copy.Items = append([]conversationMeta(nil), src.Items...)
	return &copy
}

type webServer struct {
	cfg            *cliConfig
	httpClient     *http.Client
	location       *time.Location
	store          *configStore
	hasPassword    bool
	configUnlocked bool

	configMu sync.RWMutex

	cacheMu   sync.RWMutex
	pageCache map[convPageKey]conversationPageCacheEntry

	detailMu    sync.RWMutex
	detailCache map[string]detailCacheEntry

	anyClientMu sync.Mutex
	anyClient   *anytypeClient

	notionClientMu sync.Mutex
	notionClient   *notionClient
}

type configPayload struct {
	Listen              string `json:"listen"`
	Timezone            string `json:"timezone"`
	Target              string `json:"target"`
	BaseURL             string `json:"base_url"`
	Order               string `json:"order"`
	PageSize            int    `json:"page_size"`
	MaxConversations    int    `json:"max_conversations"`
	InitialOffset       int    `json:"initial_offset"`
	IncludeArchived     bool   `json:"include_archived"`
	Token               string `json:"token"`
	DeviceID            string `json:"device_id"`
	UserAgent           string `json:"user_agent"`
	AcceptLanguage      string `json:"accept_language"`
	Referer             string `json:"referer"`
	Cookie              string `json:"cookie"`
	Origin              string `json:"origin"`
	OaiLanguage         string `json:"oai_language"`
	SecChUA             string `json:"sec_ch_ua"`
	SecChUAMobile       string `json:"sec_ch_ua_mobile"`
	SecChUAPlatform     string `json:"sec_ch_ua_platform"`
	SecFetchDest        string `json:"sec_fetch_dest"`
	SecFetchMode        string `json:"sec_fetch_mode"`
	SecFetchSite        string `json:"sec_fetch_site"`
	ChatGPTAccountID    string `json:"chatgpt_account_id"`
	OAIClientVersion    string `json:"oai_client_version"`
	Priority            string `json:"priority"`
	LogPath             string `json:"log_path"`
	AnytypeBaseURL      string `json:"anytype_base_url"`
	AnytypeVersion      string `json:"anytype_version"`
	AnytypeSpaceID      string `json:"anytype_space_id"`
	AnytypeTypeKey      string `json:"anytype_type_key"`
	AnytypeToken        string `json:"anytype_token"`
	NotionBaseURL       string `json:"notion_base_url"`
	NotionVersion       string `json:"notion_version"`
	NotionToken         string `json:"notion_token"`
	NotionParentType    string `json:"notion_parent_type"`
	NotionParentID      string `json:"notion_parent_id"`
	NotionTitleProperty string `json:"notion_title_property"`
}

type configUpdate struct {
	Listen              *string `json:"listen"`
	Timezone            *string `json:"timezone"`
	Target              *string `json:"target"`
	BaseURL             *string `json:"base_url"`
	Order               *string `json:"order"`
	PageSize            *int    `json:"page_size"`
	MaxConversations    *int    `json:"max_conversations"`
	InitialOffset       *int    `json:"initial_offset"`
	IncludeArchived     *bool   `json:"include_archived"`
	Token               *string `json:"token"`
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
}

type configStateResponse struct {
	HasPassword bool `json:"has_password"`
	Unlocked    bool `json:"unlocked"`
}

type passwordRequest struct {
	Password    string `json:"password"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

//go:embed web/dist/*
var webStatic embed.FS

var distFS fs.FS

func init() {
	var err error
	distFS, err = fs.Sub(webStatic, "web/dist")
	if err != nil {
		panic(fmt.Errorf("load embedded dist: %w", err))
	}
}

func runWebServer(ctx context.Context, httpClient *http.Client, cfg *cliConfig, token string) error {
	app, err := newWebServer(httpClient, cfg, token)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := app.Close(); cerr != nil {
			logInfo("关闭配置存储失败: %v", cerr)
		}
	}()
	server := &http.Server{
		Addr:    app.cfg.ServeAddr,
		Handler: app.routes(),
	}

	errCh := make(chan error, 1)
	go func() {
		logInfo("Web 界面已启动, 访问地址: http://%s", app.cfg.ServeAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func newWebServer(httpClient *http.Client, cfg *cliConfig, token string) (*webServer, error) {
	cfgCopy := *cfg
	cfgCopy.Token = strings.TrimSpace(token)
	ctx := context.Background()

	cfgCopy.Token = strings.TrimSpace(cfgCopy.Token)
	if cfgCopy.Token == "" {
		cfgCopy.Token = strings.TrimSpace(token)
	}
	cfgCopy.ExportTarget = normalizeExportTarget(cfgCopy.ExportTarget)
	cfgCopy.Order = normalizeOrder(cfgCopy.Order)
	cfgCopy.BaseURL = ensureBaseURL(cfgCopy.BaseURL)
	cfgCopy.PageSize = clampPageSize(cfgCopy.PageSize)
	cfgCopy.MaxConversations = nonNegative(cfgCopy.MaxConversations)
	cfgCopy.InitialOffset = nonNegative(cfgCopy.InitialOffset)
	cfgCopy.NotionParentType = sanitizeNotionParentType(cfgCopy.NotionParentType)
	cfgCopy.OutputTimezone = strings.TrimSpace(cfgCopy.OutputTimezone)
	if strings.TrimSpace(cfgCopy.UserAgent) == "" {
		cfgCopy.UserAgent = defaultUserAgent
	}
	loc := resolveLocation(cfgCopy.OutputTimezone)

	store, err := newConfigStore(cfgCopy.ConfigDBPath)
	if err != nil {
		return nil, err
	}

	app := &webServer{
		cfg:            &cfgCopy,
		httpClient:     httpClient,
		location:       loc,
		store:          store,
		hasPassword:    store.HasPassword(),
		configUnlocked: store.Unlocked(),
		pageCache:      make(map[convPageKey]conversationPageCacheEntry),
		detailCache:    make(map[string]detailCacheEntry),
	}

	if app.hasPassword {
		if secret := strings.TrimSpace(cfg.ConfigSecret); secret != "" {
			if err := store.Unlock(ctx, secret); err == nil {
				app.configUnlocked = true
			} else {
				logInfo("自动解锁配置失败: %v", err)
			}
		}
	} else if secret := strings.TrimSpace(cfg.ConfigSecret); secret != "" {
		if err := store.SetPassword(ctx, secret); err != nil {
			logInfo("初始化配置密码失败: %v", err)
		} else {
			app.hasPassword = true
			app.configUnlocked = true
			if err := store.SaveConfig(ctx, configToPayload(app.cfg)); err != nil {
				logInfo("初始化配置持久化失败: %v", err)
			}
		}
	}

	if app.hasPassword && app.configUnlocked {
		if payload, err := store.LoadConfig(ctx); err == nil {
			applyConfigPayload(app.cfg, payload)
		} else if !errors.Is(err, errConfigNotFound) {
			return nil, fmt.Errorf("加载持久化配置失败: %w", err)
		}
	}

	return app, nil
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	staticServer := http.FileServer(http.FS(distFS))
	mux.Handle("/assets/", staticServer)
	mux.Handle("/favicon.ico", staticServer)
	mux.HandleFunc("/api/config/state", s.handleConfigState)
	mux.HandleFunc("/api/config/unlock", s.handleConfigUnlock)
	mux.HandleFunc("/api/config/password", s.handleConfigPassword)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/conversations", s.handleConversationList)
	mux.HandleFunc("/api/conversations/delete", s.handleDelete)
	mux.HandleFunc("/api/conversations/", s.handleConversationDetail)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/", s.serveIndex)
	return mux
}

func (s *webServer) handleConfigState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := configStateResponse{
		HasPassword: s.hasPassword,
		Unlocked:    s.configUnlocked,
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *webServer) handleConfigUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.hasPassword {
		writeError(w, http.StatusBadRequest, "尚未设置配置密码")
		return
	}
	defer r.Body.Close()
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		writeError(w, http.StatusBadRequest, "请输入密码")
		return
	}
	if err := s.store.Unlock(r.Context(), password); err != nil {
		if errors.Is(err, errInvalidPassword) {
			writeError(w, http.StatusUnauthorized, "密码错误")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("解锁失败: %v", err))
		return
	}
	s.configUnlocked = true
	payload, err := s.store.LoadConfig(r.Context())
	if err != nil {
		if errors.Is(err, errConfigNotFound) {
			writeJSON(w, http.StatusOK, configStateResponse{HasPassword: s.hasPassword, Unlocked: s.configUnlocked})
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("加载配置失败: %v", err))
		return
	}
	s.configMu.Lock()
	applyConfigPayload(s.cfg, payload)
	s.location = resolveLocation(s.cfg.OutputTimezone)
	s.configMu.Unlock()
	writeJSON(w, http.StatusOK, payload)
}

func (s *webServer) handleConfigPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求失败: %v", err))
		return
	}
	ctx := r.Context()
	if !s.hasPassword {
		password := strings.TrimSpace(req.Password)
		if password == "" {
			writeError(w, http.StatusBadRequest, "密码不能为空")
			return
		}
		if err := s.store.SetPassword(ctx, password); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.hasPassword = true
		s.configUnlocked = true
		s.persistConfig(s.cfg)
		writeJSON(w, http.StatusOK, configStateResponse{HasPassword: true, Unlocked: true})
		return
	}

	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if oldPassword == "" || newPassword == "" {
		writeError(w, http.StatusBadRequest, "请提供旧密码和新密码")
		return
	}
	if !s.configUnlocked {
		if err := s.store.Unlock(ctx, oldPassword); err != nil {
			if errors.Is(err, errInvalidPassword) {
				writeError(w, http.StatusUnauthorized, "旧密码不正确")
				return
			}
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("解锁失败: %v", err))
			return
		}
		s.configUnlocked = true
	}

	payload := configToPayload(s.cfg)
	if err := s.store.UpdatePassword(ctx, newPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveConfig(ctx, payload); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("更新配置失败: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, configStateResponse{HasPassword: true, Unlocked: true})
}

func (s *webServer) Close() error {
	if s == nil {
		return nil
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *webServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.hasPassword && !s.configUnlocked {
			writeError(w, http.StatusForbidden, "配置已加密，请先输入密码")
			return
		}
		payload := s.currentConfigPayload()
		writeJSON(w, http.StatusOK, payload)
	case http.MethodPost:
		if !s.configUnlocked {
			if s.hasPassword {
				writeError(w, http.StatusForbidden, "配置已加密，请先解锁后再保存")
			} else {
				writeError(w, http.StatusForbidden, "请先设置配置密码，再保存修改")
			}
			return
		}
		defer r.Body.Close()
		var input configUpdate
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("解析配置失败: %v", err))
			return
		}
		payload, err := s.updateConfig(input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, payload)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *webServer) currentConfigPayload() configPayload {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return configToPayload(s.cfg)
}

func configToPayload(cfg *cliConfig) configPayload {
	if cfg == nil {
		return configPayload{}
	}
	payload := configPayload{
		Listen:              strings.TrimSpace(cfg.ServeAddr),
		Timezone:            strings.TrimSpace(cfg.OutputTimezone),
		Target:              normalizeExportTarget(cfg.ExportTarget),
		BaseURL:             strings.TrimSpace(cfg.BaseURL),
		Order:               normalizeOrder(cfg.Order),
		PageSize:            clampPageSize(cfg.PageSize),
		MaxConversations:    nonNegative(cfg.MaxConversations),
		InitialOffset:       nonNegative(cfg.InitialOffset),
		IncludeArchived:     cfg.IncludeArchived,
		Token:               strings.TrimSpace(cfg.Token),
		DeviceID:            strings.TrimSpace(cfg.DeviceID),
		UserAgent:           strings.TrimSpace(cfg.UserAgent),
		AcceptLanguage:      strings.TrimSpace(cfg.AcceptLanguage),
		Referer:             strings.TrimSpace(cfg.Referer),
		Cookie:              strings.TrimSpace(cfg.Cookie),
		Origin:              strings.TrimSpace(cfg.Origin),
		OaiLanguage:         strings.TrimSpace(cfg.OaiLanguage),
		SecChUA:             strings.TrimSpace(cfg.SecChUA),
		SecChUAMobile:       strings.TrimSpace(cfg.SecChUAMobile),
		SecChUAPlatform:     strings.TrimSpace(cfg.SecChUAPlatform),
		SecFetchDest:        strings.TrimSpace(cfg.SecFetchDest),
		SecFetchMode:        strings.TrimSpace(cfg.SecFetchMode),
		SecFetchSite:        strings.TrimSpace(cfg.SecFetchSite),
		ChatGPTAccountID:    strings.TrimSpace(cfg.ChatGPTAccountID),
		OAIClientVersion:    strings.TrimSpace(cfg.OAIClientVersion),
		Priority:            strings.TrimSpace(cfg.Priority),
		LogPath:             strings.TrimSpace(cfg.LogPath),
		AnytypeBaseURL:      strings.TrimSpace(cfg.AnytypeBaseURL),
		AnytypeVersion:      strings.TrimSpace(cfg.AnytypeVersion),
		AnytypeSpaceID:      strings.TrimSpace(cfg.AnytypeSpaceID),
		AnytypeTypeKey:      strings.TrimSpace(cfg.AnytypeTypeKey),
		AnytypeToken:        strings.TrimSpace(cfg.AnytypeToken),
		NotionBaseURL:       strings.TrimSpace(cfg.NotionBaseURL),
		NotionVersion:       strings.TrimSpace(cfg.NotionVersion),
		NotionToken:         strings.TrimSpace(cfg.NotionToken),
		NotionParentType:    sanitizeNotionParentType(cfg.NotionParentType),
		NotionParentID:      strings.TrimSpace(cfg.NotionParentID),
		NotionTitleProperty: strings.TrimSpace(cfg.NotionTitleProperty),
	}
	if payload.BaseURL == "" {
		payload.BaseURL = defaultBaseURL
	}
	return payload
}

func applyConfigPayload(cfg *cliConfig, payload configPayload) {
	if cfg == nil {
		return
	}
	if listen := strings.TrimSpace(payload.Listen); listen != "" {
		cfg.ServeAddr = listen
	}
	if tz := strings.TrimSpace(payload.Timezone); tz != "" {
		cfg.OutputTimezone = tz
	}
	cfg.ExportTarget = normalizeExportTarget(payload.Target)
	cfg.BaseURL = strings.TrimSpace(payload.BaseURL)
	cfg.Order = payload.Order
	if payload.PageSize > 0 {
		cfg.PageSize = payload.PageSize
	}
	cfg.MaxConversations = payload.MaxConversations
	cfg.InitialOffset = payload.InitialOffset
	cfg.IncludeArchived = payload.IncludeArchived
	cfg.Token = strings.TrimSpace(payload.Token)
	cfg.DeviceID = strings.TrimSpace(payload.DeviceID)
	cfg.UserAgent = strings.TrimSpace(payload.UserAgent)
	cfg.AcceptLanguage = strings.TrimSpace(payload.AcceptLanguage)
	cfg.Referer = strings.TrimSpace(payload.Referer)
	cfg.Cookie = strings.TrimSpace(payload.Cookie)
	cfg.Origin = strings.TrimSpace(payload.Origin)
	cfg.OaiLanguage = strings.TrimSpace(payload.OaiLanguage)
	cfg.SecChUA = strings.TrimSpace(payload.SecChUA)
	cfg.SecChUAMobile = strings.TrimSpace(payload.SecChUAMobile)
	cfg.SecChUAPlatform = strings.TrimSpace(payload.SecChUAPlatform)
	cfg.SecFetchDest = strings.TrimSpace(payload.SecFetchDest)
	cfg.SecFetchMode = strings.TrimSpace(payload.SecFetchMode)
	cfg.SecFetchSite = strings.TrimSpace(payload.SecFetchSite)
	cfg.ChatGPTAccountID = strings.TrimSpace(payload.ChatGPTAccountID)
	cfg.OAIClientVersion = strings.TrimSpace(payload.OAIClientVersion)
	cfg.Priority = strings.TrimSpace(payload.Priority)
	cfg.LogPath = strings.TrimSpace(payload.LogPath)
	cfg.AnytypeBaseURL = strings.TrimSpace(payload.AnytypeBaseURL)
	cfg.AnytypeVersion = strings.TrimSpace(payload.AnytypeVersion)
	cfg.AnytypeSpaceID = strings.TrimSpace(payload.AnytypeSpaceID)
	cfg.AnytypeTypeKey = strings.TrimSpace(payload.AnytypeTypeKey)
	cfg.AnytypeToken = strings.TrimSpace(payload.AnytypeToken)
	cfg.NotionBaseURL = strings.TrimSpace(payload.NotionBaseURL)
	cfg.NotionVersion = strings.TrimSpace(payload.NotionVersion)
	cfg.NotionToken = strings.TrimSpace(payload.NotionToken)
	cfg.NotionParentType = sanitizeNotionParentType(payload.NotionParentType)
	cfg.NotionParentID = strings.TrimSpace(payload.NotionParentID)
	cfg.NotionTitleProperty = strings.TrimSpace(payload.NotionTitleProperty)
}

func (s *webServer) updateConfig(input configUpdate) (configPayload, error) {
	s.configMu.Lock()
	cfg := s.cfg

	if input.Listen != nil {
		cfg.ServeAddr = strings.TrimSpace(*input.Listen)
	}
	if input.Timezone != nil {
		cfg.OutputTimezone = strings.TrimSpace(*input.Timezone)
	}
	if input.Target != nil {
		cfg.ExportTarget = normalizeExportTarget(*input.Target)
	}
	if input.BaseURL != nil {
		cfg.BaseURL = ensureBaseURL(*input.BaseURL)
	}
	if input.Order != nil {
		cfg.Order = normalizeOrder(*input.Order)
	}
	if input.PageSize != nil {
		cfg.PageSize = clampPageSize(*input.PageSize)
	}
	if input.MaxConversations != nil {
		cfg.MaxConversations = nonNegative(*input.MaxConversations)
	}
	if input.InitialOffset != nil {
		cfg.InitialOffset = nonNegative(*input.InitialOffset)
	}
	if input.IncludeArchived != nil {
		cfg.IncludeArchived = *input.IncludeArchived
	}
	if input.Token != nil {
		cfg.Token = strings.TrimSpace(*input.Token)
	}
	if input.DeviceID != nil {
		cfg.DeviceID = strings.TrimSpace(*input.DeviceID)
	}
	if input.UserAgent != nil {
		cfg.UserAgent = strings.TrimSpace(*input.UserAgent)
	}
	if input.AcceptLanguage != nil {
		cfg.AcceptLanguage = strings.TrimSpace(*input.AcceptLanguage)
	}
	if input.Referer != nil {
		cfg.Referer = strings.TrimSpace(*input.Referer)
	}
	if input.Cookie != nil {
		cfg.Cookie = strings.TrimSpace(*input.Cookie)
	}
	if input.Origin != nil {
		cfg.Origin = strings.TrimSpace(*input.Origin)
	}
	if input.OaiLanguage != nil {
		cfg.OaiLanguage = strings.TrimSpace(*input.OaiLanguage)
	}
	if input.SecChUA != nil {
		cfg.SecChUA = strings.TrimSpace(*input.SecChUA)
	}
	if input.SecChUAMobile != nil {
		cfg.SecChUAMobile = strings.TrimSpace(*input.SecChUAMobile)
	}
	if input.SecChUAPlatform != nil {
		cfg.SecChUAPlatform = strings.TrimSpace(*input.SecChUAPlatform)
	}
	if input.SecFetchDest != nil {
		cfg.SecFetchDest = strings.TrimSpace(*input.SecFetchDest)
	}
	if input.SecFetchMode != nil {
		cfg.SecFetchMode = strings.TrimSpace(*input.SecFetchMode)
	}
	if input.SecFetchSite != nil {
		cfg.SecFetchSite = strings.TrimSpace(*input.SecFetchSite)
	}
	if input.ChatGPTAccountID != nil {
		cfg.ChatGPTAccountID = strings.TrimSpace(*input.ChatGPTAccountID)
	}
	if input.OAIClientVersion != nil {
		cfg.OAIClientVersion = strings.TrimSpace(*input.OAIClientVersion)
	}
	if input.Priority != nil {
		cfg.Priority = strings.TrimSpace(*input.Priority)
	}
	if input.LogPath != nil {
		cfg.LogPath = strings.TrimSpace(*input.LogPath)
	}
	if input.AnytypeBaseURL != nil {
		cfg.AnytypeBaseURL = strings.TrimSpace(*input.AnytypeBaseURL)
	}
	if input.AnytypeVersion != nil {
		cfg.AnytypeVersion = strings.TrimSpace(*input.AnytypeVersion)
	}
	if input.AnytypeSpaceID != nil {
		cfg.AnytypeSpaceID = strings.TrimSpace(*input.AnytypeSpaceID)
	}
	if input.AnytypeTypeKey != nil {
		cfg.AnytypeTypeKey = strings.TrimSpace(*input.AnytypeTypeKey)
	}
	if input.AnytypeToken != nil {
		cfg.AnytypeToken = strings.TrimSpace(*input.AnytypeToken)
	}
	if input.NotionBaseURL != nil {
		cfg.NotionBaseURL = strings.TrimSpace(*input.NotionBaseURL)
	}
	if input.NotionVersion != nil {
		cfg.NotionVersion = strings.TrimSpace(*input.NotionVersion)
	}
	if input.NotionToken != nil {
		cfg.NotionToken = strings.TrimSpace(*input.NotionToken)
	}
	if input.NotionParentType != nil {
		cfg.NotionParentType = sanitizeNotionParentType(*input.NotionParentType)
	}
	if input.NotionParentID != nil {
		cfg.NotionParentID = strings.TrimSpace(*input.NotionParentID)
	}
	if input.NotionTitleProperty != nil {
		cfg.NotionTitleProperty = strings.TrimSpace(*input.NotionTitleProperty)
	}

	s.location = resolveLocation(cfg.OutputTimezone)
	cfgCopy := *cfg
	payload := configToPayload(cfg)
	s.configMu.Unlock()

	s.invalidateConversationCache()
	s.clearDetailCache()
	s.resetExportClients()
	s.persistConfig(&cfgCopy)

	return payload, nil
}

func (s *webServer) persistConfig(cfg *cliConfig) {
	if s == nil || s.store == nil || cfg == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.store.SaveConfig(ctx, configToPayload(cfg)); err != nil {
		if errors.Is(err, errStoreLocked) || errors.Is(err, errPasswordNotSet) {
			logInfo("配置未持久化: %v", err)
		} else {
			logInfo("配置持久化失败: %v", err)
		}
	}
}

func normalizeExportTarget(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case exportTargetNotion:
		return exportTargetNotion
	default:
		return exportTargetAnytype
	}
}

func normalizeOrder(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "created":
		return "created"
	default:
		return "updated"
	}
}

func ensureBaseURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultBaseURL
	}
	return trimmed
}

func clampPageSize(value int) int {
	if value <= 0 {
		value = 20
	}
	if value > 100 {
		value = 100
	}
	return value
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func sanitizeNotionParentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "page":
		return "page"
	case "database":
		return "database"
	default:
		return ""
	}
}

func (s *webServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	indexHTML, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("加载前端页面失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(indexHTML); err != nil {
		logInfo("输出首页失败: %v", err)
	}
}

func (s *webServer) handleConversationList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query()
	force := query.Get("refresh") == "1"

	cfg := s.configSnapshot()
	loc := s.locationSnapshot()

	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit <= 0 {
		limit = cfg.PageSize
	}
	limit = clampPageSize(limit)

	page, err := s.getConversationPage(r.Context(), offset, limit, force)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("获取对话列表失败: %v", err))
		return
	}

	items := make([]apiConversationItem, 0, len(page.Items))
	for _, meta := range page.Items {
		items = append(items, apiConversationItem{
			ID:         meta.ID,
			Title:      firstNonEmpty(meta.Title, "(未命名对话)"),
			CreateTime: formatTimestamp(meta.CreateTime.Float64(), loc),
			UpdateTime: formatTimestamp(meta.UpdateTime.Float64(), loc),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":    items,
		"total":    page.Total,
		"has_more": page.HasMore,
		"offset":   page.Offset,
		"limit":    page.Limit,
	})
}

func (s *webServer) handleConversationDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	loc := s.locationSnapshot()
	id := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	force := r.URL.Query().Get("refresh") == "1"
	conv, err := s.loadExportConversation(r.Context(), id, force)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("获取对话详情失败: %v", err))
		return
	}
	resp := apiConversationDetail{
		ID:         conv.ID,
		Title:      firstNonEmpty(conv.Title, "(未命名对话)"),
		CreateTime: formatTimestamp(conv.CreateTime, loc),
		UpdateTime: formatTimestamp(conv.UpdateTime, loc),
	}
	resp.Messages = make([]apiMessage, 0, len(conv.Messages))
	for _, msg := range conv.Messages {
		resp.Messages = append(resp.Messages, apiMessage{
			Role:      msg.Role,
			Timestamp: s.formatMessageTimestamp(msg),
			Text:      msg.Text,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *webServer) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "请选择至少一条对话")
		return
	}

	ctx := r.Context()
	seen := make(map[string]struct{})
	var exports []exportConversation
	var skipped []string

	for _, rawID := range req.IDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		conv, err := s.loadExportConversation(ctx, id, true)
		if err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("获取对话 %s 详情失败: %v", id, err))
			return
		}
		if len(conv.Messages) == 0 {
			skipped = append(skipped, id)
			continue
		}
		exports = append(exports, conv)
	}

	if len(exports) == 0 {
		writeError(w, http.StatusBadRequest, "选中的对话没有可导出的消息")
		return
	}

	cfg := s.configSnapshot()
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = cfg.ExportTarget
	}
	target = normalizeExportTarget(target)

	logInfo("Web 导入触发: 选中=%d 有效=%d 目标=%s", len(req.IDs), len(exports), target)

	var (
		created     int
		pages       []string
		syncErr     error
		targetLabel = target
	)

	switch target {
	case exportTargetAnytype:
		targetLabel = "Anytype"
		client, err := s.resolveAnytypeClient()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, syncErr = syncConversationsToAnytype(ctx, client, exports, cfg.OutputTimezone)
	case exportTargetNotion:
		targetLabel = "Notion"
		client, err := s.resolveNotionClient()
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, pages, syncErr = syncConversationsToNotion(ctx, client, exports, cfg.OutputTimezone)
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("不支持的导出目标: %s", target))
		return
	}

	if syncErr != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("导入 %s 失败: %v", targetLabel, syncErr))
		return
	}

	response := map[string]interface{}{
		"created": created,
		"skipped": skipped,
		"target":  target,
	}
	if len(pages) > 0 {
		response["pages"] = pages
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *webServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := s.configSnapshot()
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, "缺少 OpenAI Token, 请先在配置页填写")
		return
	}
	var req deleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "请选择至少一条对话")
		return
	}

	ctx := r.Context()
	seen := make(map[string]struct{})
	var deleted []string

	for _, rawID := range req.IDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		if err := deleteConversation(ctx, s.httpClient, cfg, token, id); err != nil {
			writeError(w, http.StatusBadGateway, fmt.Sprintf("删除对话 %s 失败: %v", id, err))
			return
		}
		s.removeDetailCache(id)
		deleted = append(deleted, id)
	}

	if len(deleted) == 0 {
		writeError(w, http.StatusBadRequest, "没有有效的对话可删除")
		return
	}

	s.invalidateConversationCache()
	logInfo("Web 删除触发: 删除成功=%d", len(deleted))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
		"count":   len(deleted),
	})
}

func (s *webServer) getConversationPage(ctx context.Context, offset, limit int, force bool) (*conversationListResponse, error) {
	key := convPageKey{offset: offset, limit: limit}

	if !force {
		s.cacheMu.RLock()
		if entry, ok := s.pageCache[key]; ok && time.Since(entry.fetched) < conversationCacheTTL {
			page := cloneConversationPage(entry.data)
			s.cacheMu.RUnlock()
			return page, nil
		}
		s.cacheMu.RUnlock()
	}

	cfg := s.configSnapshot()
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("缺少 OpenAI Token, 请先在配置页填写")
	}

	page, err := fetchConversationPage(ctx, s.httpClient, cfg, token, offset, limit)
	if err != nil {
		return nil, err
	}

	cloned := cloneConversationPage(page)

	s.cacheMu.Lock()
	s.pageCache[key] = conversationPageCacheEntry{
		data:    cloneConversationPage(page),
		fetched: time.Now(),
	}
	s.cacheMu.Unlock()

	return cloned, nil
}

func (s *webServer) loadExportConversation(ctx context.Context, id string, force bool) (exportConversation, error) {
	if strings.TrimSpace(id) == "" {
		return exportConversation{}, errors.New("缺少对话 ID")
	}

	if force {
		s.detailMu.Lock()
		delete(s.detailCache, id)
		s.detailMu.Unlock()
	}

	if !force {
		s.detailMu.RLock()
		if entry, ok := s.detailCache[id]; ok && time.Since(entry.fetched) < detailCacheTTL {
			export := entry.export
			s.detailMu.RUnlock()
			return export, nil
		}
		s.detailMu.RUnlock()
	}

	cfg := s.configSnapshot()
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return exportConversation{}, errors.New("缺少 OpenAI Token, 请先在配置页填写")
	}

	detail, err := fetchConversationDetail(ctx, s.httpClient, cfg, token, id)
	if err != nil {
		return exportConversation{}, err
	}

	meta := conversationMeta{
		ID:         firstNonEmpty(detail.ID, id),
		Title:      detail.Title,
		CreateTime: detail.CreateTime,
		UpdateTime: detail.UpdateTime,
	}

	if strings.TrimSpace(meta.Title) == "" {
		if cached, ok := s.lookupConversationMeta(id); ok {
			meta = cached
		}
	}

	export := buildExportConversation(meta, detail)

	s.detailMu.Lock()
	s.detailCache[id] = detailCacheEntry{
		export:  export,
		fetched: time.Now(),
	}
	s.detailMu.Unlock()

	return export, nil
}

func (s *webServer) lookupConversationMeta(id string) (conversationMeta, bool) {
	if strings.TrimSpace(id) == "" {
		return conversationMeta{}, false
	}
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	for _, entry := range s.pageCache {
		if entry.data == nil {
			continue
		}
		if time.Since(entry.fetched) > conversationCacheTTL {
			continue
		}
		for _, item := range entry.data.Items {
			if item.ID == id {
				return item, true
			}
		}
	}
	return conversationMeta{}, false
}

func (s *webServer) invalidateConversationCache() {
	s.cacheMu.Lock()
	s.pageCache = make(map[convPageKey]conversationPageCacheEntry)
	s.cacheMu.Unlock()
}

func (s *webServer) removeDetailCache(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	s.detailMu.Lock()
	delete(s.detailCache, id)
	s.detailMu.Unlock()
}

func (s *webServer) clearDetailCache() {
	s.detailMu.Lock()
	s.detailCache = make(map[string]detailCacheEntry)
	s.detailMu.Unlock()
}

func (s *webServer) resetExportClients() {
	s.anyClientMu.Lock()
	s.anyClient = nil
	s.anyClientMu.Unlock()

	s.notionClientMu.Lock()
	s.notionClient = nil
	s.notionClientMu.Unlock()
}

func (s *webServer) configSnapshot() *cliConfig {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	copy := *s.cfg
	return &copy
}

func (s *webServer) locationSnapshot() *time.Location {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.location
}

func (s *webServer) formatMessageTimestamp(msg exportMessage) string {
	loc := s.locationSnapshot()
	if msg.CreateTime > 0 {
		return formatTimestamp(msg.CreateTime, loc)
	}
	if msg.UpdateTime > 0 {
		return formatTimestamp(msg.UpdateTime, loc)
	}
	return "-"
}

func (s *webServer) resolveAnytypeClient() (*anytypeClient, error) {
	cfg := s.configSnapshot()
	s.anyClientMu.Lock()
	defer s.anyClientMu.Unlock()
	if s.anyClient != nil {
		return s.anyClient, nil
	}
	client, err := newAnytypeClient(cfg, s.httpClient)
	if err != nil {
		return nil, err
	}
	s.anyClient = client
	return client, nil
}

func (s *webServer) resolveNotionClient() (*notionClient, error) {
	cfg := s.configSnapshot()
	s.notionClientMu.Lock()
	defer s.notionClientMu.Unlock()
	if s.notionClient != nil {
		return s.notionClient, nil
	}
	client, err := newNotionClient(cfg, s.httpClient)
	if err != nil {
		return nil, err
	}
	s.notionClient = client
	return client, nil
}

type apiConversationItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
}

type apiMessage struct {
	Role      string `json:"role"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type apiConversationDetail struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	CreateTime string       `json:"create_time"`
	UpdateTime string       `json:"update_time"`
	Messages   []apiMessage `json:"messages"`
}

type importRequest struct {
	IDs    []string `json:"ids"`
	Target string   `json:"target"`
}

type deleteRequest struct {
	IDs []string `json:"ids"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logInfo("写入 JSON 响应失败: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	if status < 400 {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": message})
}
