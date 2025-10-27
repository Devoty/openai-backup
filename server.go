package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	conversationCacheTTL = 30 * time.Second
	detailCacheTTL       = 5 * time.Minute
)

type conversationCache struct {
	items   []conversationMeta
	fetched time.Time
}

type detailCacheEntry struct {
	export  exportConversation
	fetched time.Time
}

type webServer struct {
	cfg        *cliConfig
	httpClient *http.Client
	token      string
	location   *time.Location

	cacheMu   sync.RWMutex
	convCache conversationCache

	detailMu    sync.RWMutex
	detailCache map[string]detailCacheEntry

	anyClientMu sync.Mutex
	anyClient   *anytypeClient
}

var indexTemplate = template.Must(template.New("index").Parse(indexPageHTML))

const indexPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<title>ChatGPT 对话导出 · Web 界面</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		:root {
			color-scheme: light;
		}
		body {
			margin: 0;
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
			background: #f6f7fb;
			color: #1f2933;
		}
		header {
			background: #2f3a4f;
			color: #ffffff;
			padding: 18px 28px;
			box-shadow: 0 2px 6px rgba(0, 0, 0, 0.2);
		}
		header h1 {
			margin: 0 0 6px;
			font-size: 1.6rem;
			font-weight: 600;
		}
		header .meta {
			display: flex;
			gap: 16px;
			align-items: center;
			flex-wrap: wrap;
			font-size: 0.95rem;
		}
		main {
			padding: 24px;
			display: grid;
			grid-template-columns: minmax(360px, 1fr) minmax(380px, 1fr);
			gap: 24px;
		}
		@media (max-width: 980px) {
			main {
				grid-template-columns: 1fr;
			}
		}
		section {
			background: #ffffff;
			border-radius: 12px;
			box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
			padding: 18px 20px;
			display: flex;
			flex-direction: column;
			min-height: 320px;
		}
		section h2 {
			margin: 0 0 14px;
			font-size: 1.2rem;
			display: flex;
			align-items: center;
			justify-content: space-between;
		}
		.table-wrapper {
			overflow: auto;
			max-height: calc(100vh - 240px);
			border-radius: 10px;
			border: 1px solid #d9e2ec;
		}
		table {
			width: 100%;
			border-collapse: collapse;
			min-width: 640px;
		}
		th, td {
			padding: 10px 12px;
			text-align: left;
			border-bottom: 1px solid #e4ecf7;
			font-size: 0.95rem;
		}
		th {
			background: #f1f5f9;
			font-weight: 600;
			color: #334155;
			position: sticky;
			top: 0;
			z-index: 1;
		}
		tbody tr:hover {
			background: #f8fafc;
		}
		.title-cell {
			font-weight: 600;
			color: #1f2937;
		}
		button {
			border: none;
			border-radius: 6px;
			padding: 6px 12px;
			cursor: pointer;
			background: #2563eb;
			color: #ffffff;
			font-size: 0.9rem;
			transition: background 0.2s ease, transform 0.1s ease;
		}
		button[disabled] {
			opacity: 0.6;
			cursor: not-allowed;
		}
		button:hover:not([disabled]) {
			background: #1d4ed8;
		}
		.inline-button {
			font-size: 0.85rem;
			padding: 4px 10px;
		}
		.controls {
			display: flex;
			gap: 10px;
			align-items: center;
			flex-wrap: wrap;
		}
		#message {
			margin-top: 12px;
			padding: 10px 12px;
			border-radius: 6px;
			background: rgba(255, 255, 255, 0.15);
			color: #ffffff;
			min-height: 1.2rem;
		}
		#message.error {
			background: #fee2e2;
			color: #7f1d1d;
		}
		#message.info {
			background: #dcfce7;
			color: #166534;
		}
		.preview-content {
			flex: 1;
			overflow-y: auto;
			border: 1px solid #d9e2ec;
			border-radius: 10px;
			padding: 14px;
			background: #f8fafc;
		}
		.preview-meta {
			font-size: 0.9rem;
			color: #475569;
			margin-bottom: 12px;
			line-height: 1.5;
		}
		.message {
			background: #ffffff;
			border-radius: 8px;
			padding: 12px;
			margin-bottom: 14px;
			box-shadow: 0 10px 18px rgba(15, 23, 42, 0.04);
			border-left: 4px solid #64748b;
		}
		.message.role-user {
			border-left-color: #f59e0b;
			background: #fff7ed;
		}
		.message.role-assistant {
			border-left-color: #2563eb;
			background: #eef2ff;
		}
		.message.role-system {
			border-left-color: #0f766e;
			background: #ecfdf5;
		}
		.message-header {
			font-weight: 600;
			margin-bottom: 8px;
			color: #1f2937;
		}
		.message pre {
			margin: 0;
			white-space: pre-wrap;
			word-break: break-word;
			font-family: ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
		}
		.empty-placeholder {
			text-align: center;
			color: #64748b;
			margin-top: 40px;
		}
	</style>
</head>
<body>
	<header>
		<h1>ChatGPT 对话导出</h1>
		<div class="meta">
			<span>监听地址: {{.Listen}}</span>
			<span>输出时区: {{.Timezone}}</span>
			<div class="controls">
				<button id="reload-button" type="button">刷新列表</button>
				<button id="import-button" type="button">导入所选</button>
			</div>
		</div>
		<div id="message"></div>
	</header>
	<main>
		<section>
			<h2>对话列表 <span id="total-count">(0)</span></h2>
			<div class="table-wrapper">
				<table>
					<thead>
						<tr>
							<th style="width: 48px;">选择</th>
							<th>标题</th>
							<th style="width: 140px;">创建时间</th>
							<th style="width: 140px;">最近更新</th>
							<th style="width: 90px;">操作</th>
						</tr>
					</thead>
					<tbody id="conversation-body"></tbody>
				</table>
			</div>
		</section>
		<section>
			<h2>对话预览</h2>
			<div class="preview-content">
				<h3 id="preview-title">请选择左侧的对话查看详情</h3>
				<div class="preview-meta" id="preview-meta"></div>
				<div id="preview-messages"></div>
			</div>
		</section>
	</main>
	<script>
		document.addEventListener('DOMContentLoaded', () => {
			const tableBody = document.getElementById('conversation-body');
			const totalCount = document.getElementById('total-count');
			const previewTitle = document.getElementById('preview-title');
			const previewMeta = document.getElementById('preview-meta');
			const previewMessages = document.getElementById('preview-messages');
			const messageBox = document.getElementById('message');
			const importButton = document.getElementById('import-button');
			const reloadButton = document.getElementById('reload-button');

			const selected = new Set();
			let conversations = [];

			function setMessage(text, isError = false) {
				messageBox.textContent = text || '';
				messageBox.className = '';
				if (!text) {
					return;
				}
				messageBox.classList.add(isError ? 'error' : 'info');
				if (!isError) {
					setTimeout(() => {
						if (messageBox.textContent === text) {
							messageBox.textContent = '';
							messageBox.className = '';
						}
					}, 3200);
				}
			}

			function renderList(items) {
				tableBody.innerHTML = '';
				totalCount.textContent = '(' + items.length + ')';
				if (items.length === 0) {
					const emptyRow = document.createElement('tr');
					const emptyCell = document.createElement('td');
					emptyCell.colSpan = 5;
					emptyCell.innerHTML = '<div class="empty-placeholder">暂未获取到对话记录</div>';
					emptyRow.appendChild(emptyCell);
					tableBody.appendChild(emptyRow);
					return;
				}
				items.forEach(item => {
					const tr = document.createElement('tr');

					const selectCell = document.createElement('td');
					const checkbox = document.createElement('input');
					checkbox.type = 'checkbox';
					checkbox.checked = selected.has(item.id);
					checkbox.addEventListener('change', () => {
						if (checkbox.checked) {
							selected.add(item.id);
						} else {
							selected.delete(item.id);
						}
					});
					selectCell.appendChild(checkbox);
					tr.appendChild(selectCell);

					const titleCell = document.createElement('td');
					titleCell.className = 'title-cell';
					titleCell.textContent = item.title || '(未命名对话)';
					tr.appendChild(titleCell);

					const createdCell = document.createElement('td');
					createdCell.textContent = item.create_time;
					tr.appendChild(createdCell);

					const updatedCell = document.createElement('td');
					updatedCell.textContent = item.update_time;
					tr.appendChild(updatedCell);

					const actionCell = document.createElement('td');
					const previewButton = document.createElement('button');
					previewButton.className = 'inline-button';
					previewButton.textContent = '预览';
					previewButton.addEventListener('click', () => {
						previewConversation(item.id);
					});
					actionCell.appendChild(previewButton);
					tr.appendChild(actionCell);

					tr.addEventListener('dblclick', () => {
						checkbox.checked = !checkbox.checked;
						if (checkbox.checked) {
							selected.add(item.id);
						} else {
							selected.delete(item.id);
						}
					});

					tableBody.appendChild(tr);
				});
			}

			async function loadConversations(force = false) {
				const url = force ? '/api/conversations?refresh=1' : '/api/conversations';
				try {
					setMessage(force ? '正在刷新对话列表…' : '正在加载对话列表…', false);
					const response = await fetch(url, {
						headers: { 'Accept': 'application/json' }
					});
					const payload = await response.json().catch(() => ({}));
					if (!response.ok) {
						throw new Error(payload.error || response.statusText);
					}
					conversations = Array.isArray(payload.items) ? payload.items : [];
					renderList(conversations);
					setMessage('已加载 ' + conversations.length + ' 条对话');
				} catch (error) {
					setMessage(error.message || '加载列表失败', true);
				}
			}

			async function previewConversation(id) {
				if (!id) {
					return;
				}
				try {
					setMessage('正在加载对话预览…', false);
					const response = await fetch('/api/conversations/' + encodeURIComponent(id), {
						headers: { 'Accept': 'application/json' }
					});
					const payload = await response.json().catch(() => ({}));
					if (!response.ok) {
						throw new Error(payload.error || response.statusText);
					}

					previewTitle.textContent = payload.title || payload.id || '(未命名对话)';
					previewMeta.textContent = 'ID: ' + (payload.id || '-') + ' · 创建: ' + (payload.create_time || '-') + ' · 最近更新: ' + (payload.update_time || '-');
					previewMessages.innerHTML = '';

					if (!Array.isArray(payload.messages) || payload.messages.length === 0) {
						const placeholder = document.createElement('div');
						placeholder.className = 'empty-placeholder';
						placeholder.textContent = '这条对话没有可展示的消息。';
						previewMessages.appendChild(placeholder);
					} else {
						payload.messages.forEach(msg => {
							const container = document.createElement('div');
							const roleClass = msg.role ? 'role-' + msg.role.toLowerCase() : 'role-unknown';
							container.classList.add('message', roleClass);

							const header = document.createElement('div');
							header.className = 'message-header';
							const roleLabel = msg.role ? msg.role.toUpperCase() : 'UNKNOWN';
							header.textContent = roleLabel + ' · ' + (msg.timestamp || '-');
							container.appendChild(header);

							const body = document.createElement('pre');
							body.textContent = msg.text || '';
							container.appendChild(body);

							previewMessages.appendChild(container);
						});
					}
					setMessage('预览已更新');
				} catch (error) {
					setMessage(error.message || '加载对话详情失败', true);
				}
			}

			async function importSelected() {
				const ids = Array.from(selected);
				if (ids.length === 0) {
					setMessage('请先在列表中勾选需要导入的对话', true);
					return;
				}
				importButton.disabled = true;
				const originalLabel = importButton.textContent;
				importButton.textContent = '导入中…';
				try {
					const response = await fetch('/api/import', {
						method: 'POST',
						headers: {
							'Content-Type': 'application/json',
							'Accept': 'application/json'
						},
						body: JSON.stringify({ ids })
					});
					const payload = await response.json().catch(() => ({}));
					if (!response.ok) {
						throw new Error(payload.error || response.statusText);
					}
					let message = '成功导入 ' + (payload.created || 0) + ' 条对话';
					if (Array.isArray(payload.skipped) && payload.skipped.length > 0) {
						message += '，跳过 ' + payload.skipped.length + ' 条（无有效消息）。';
					}
					setMessage(message);
				} catch (error) {
					setMessage(error.message || '导入失败', true);
				} finally {
					importButton.disabled = false;
					importButton.textContent = originalLabel;
				}
			}

			reloadButton.addEventListener('click', () => loadConversations(true));
			importButton.addEventListener('click', importSelected);

			loadConversations();
		});
	</script>
</body>
</html>`

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
		detailCache: make(map[string]detailCacheEntry),
	}
}

func (s *webServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveIndex)
	mux.HandleFunc("/api/conversations", s.handleConversationList)
	mux.HandleFunc("/api/conversations/", s.handleConversationDetail)
	mux.HandleFunc("/api/import", s.handleImport)
	return mux
}

func (s *webServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	data := map[string]string{
		"Listen":   s.cfg.ServeAddr,
		"Timezone": s.cfg.OutputTimezone,
	}
	if err := indexTemplate.Execute(w, data); err != nil {
		logInfo("渲染首页失败: %v", err)
	}
}

func (s *webServer) handleConversationList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	force := r.URL.Query().Get("refresh") == "1"
	convs, err := s.getConversationList(r.Context(), force)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("获取对话列表失败: %v", err))
		return
	}
	items := make([]apiConversationItem, 0, len(convs))
	for _, meta := range convs {
		items = append(items, apiConversationItem{
			ID:         meta.ID,
			Title:      firstNonEmpty(meta.Title, "(未命名对话)"),
			CreateTime: formatTimestamp(meta.CreateTime.Float64(), s.location),
			UpdateTime: formatTimestamp(meta.UpdateTime.Float64(), s.location),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
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

func (s *webServer) getConversationList(ctx context.Context, force bool) ([]conversationMeta, error) {
	s.cacheMu.RLock()
	cached := s.convCache
	s.cacheMu.RUnlock()

	if !force && len(cached.items) > 0 && time.Since(cached.fetched) < conversationCacheTTL {
		return append([]conversationMeta(nil), cached.items...), nil
	}

	items, err := fetchAllConversations(ctx, s.httpClient, s.cfg, s.token)
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.convCache = conversationCache{
		items:   append([]conversationMeta(nil), items...),
		fetched: time.Now(),
	}
	s.cacheMu.Unlock()

	return append([]conversationMeta(nil), items...), nil
}

func (s *webServer) loadExportConversation(ctx context.Context, id string, force bool) (exportConversation, error) {
	if id == "" {
		return exportConversation{}, errors.New("缺少对话 ID")
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
		if list, err := s.getConversationList(ctx, false); err == nil {
			for _, item := range list {
				if item.ID == id {
					meta = item
					break
				}
			}
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
