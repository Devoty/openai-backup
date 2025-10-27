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
	cfg        *cliConfig
	httpClient *http.Client
	token      string
	location   *time.Location

	cacheMu   sync.RWMutex
	pageCache map[convPageKey]conversationPageCacheEntry

	detailMu    sync.RWMutex
	detailCache map[string]detailCacheEntry

	anyClientMu sync.Mutex
	anyClient   *anytypeClient
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
	app := newWebServer(httpClient, cfg, token)
	server := &http.Server{
		Addr:    cfg.ServeAddr,
		Handler: app.routes(),
	}

	errCh := make(chan error, 1)
	go func() {
		logInfo("Web 界面已启动, 访问地址: http://%s", cfg.ServeAddr)
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

func newWebServer(httpClient *http.Client, cfg *cliConfig, token string) *webServer {
	loc := resolveLocation(cfg.OutputTimezone)
	return &webServer{
		cfg:         cfg,
		httpClient:  httpClient,
		token:       token,
		location:    loc,
		pageCache:   make(map[convPageKey]conversationPageCacheEntry),
		detailCache: make(map[string]detailCacheEntry),
	}
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	staticServer := http.FileServer(http.FS(distFS))
	mux.Handle("/assets/", staticServer)
	mux.Handle("/favicon.ico", staticServer)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/conversations", s.handleConversationList)
	mux.HandleFunc("/api/conversations/delete", s.handleDelete)
	mux.HandleFunc("/api/conversations/", s.handleConversationDetail)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/", s.serveIndex)
	return mux
}

func (s *webServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	payload := map[string]string{
		"listen":   strings.TrimSpace(s.cfg.ServeAddr),
		"timezone": strings.TrimSpace(s.cfg.OutputTimezone),
	}
	writeJSON(w, http.StatusOK, payload)
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

	offset, err := strconv.Atoi(query.Get("offset"))
	if err != nil || offset < 0 {
		offset = 0
	}
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit <= 0 {
		limit = s.cfg.PageSize
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

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
			CreateTime: formatTimestamp(meta.CreateTime.Float64(), s.location),
			UpdateTime: formatTimestamp(meta.UpdateTime.Float64(), s.location),
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
		CreateTime: formatTimestamp(conv.CreateTime, s.location),
		UpdateTime: formatTimestamp(conv.UpdateTime, s.location),
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

	client, err := s.resolveAnytypeClient()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	logInfo("Web 导入触发: 选中=%d 有效=%d", len(req.IDs), len(exports))
	created, err := syncConversationsToAnytype(ctx, client, exports, s.cfg.OutputTimezone)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("导入 Anytype 失败: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"created": created,
		"skipped": skipped,
	})
}

func (s *webServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

		if err := deleteConversation(ctx, s.httpClient, s.cfg, s.token, id); err != nil {
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

	page, err := fetchConversationPage(ctx, s.httpClient, s.cfg, s.token, offset, limit)
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

	detail, err := fetchConversationDetail(ctx, s.httpClient, s.cfg, s.token, id)
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

func (s *webServer) formatMessageTimestamp(msg exportMessage) string {
	if msg.CreateTime > 0 {
		return formatTimestamp(msg.CreateTime, s.location)
	}
	if msg.UpdateTime > 0 {
		return formatTimestamp(msg.UpdateTime, s.location)
	}
	return "-"
}

func (s *webServer) resolveAnytypeClient() (*anytypeClient, error) {
	s.anyClientMu.Lock()
	defer s.anyClientMu.Unlock()
	if s.anyClient != nil {
		return s.anyClient, nil
	}
	client, err := newAnytypeClient(s.cfg, s.httpClient)
	if err != nil {
		return nil, err
	}
	s.anyClient = client
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
	IDs []string `json:"ids"`
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
