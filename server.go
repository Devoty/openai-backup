package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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

var indexTemplate = template.Must(template.New("index").Parse(indexPageHTML))

const indexPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<title>ChatGPT 对话导出 · Web 界面</title>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<style>
		html, body {
			height: 100%;
		}
		:root {
			color-scheme: light;
		}
		body {
			margin: 0;
			min-height: 100%;
			display: flex;
			flex-direction: column;
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
		main {
			flex: 1;
			padding: 24px;
			display: grid;
			grid-template-columns: minmax(360px, 1fr) minmax(380px, 1fr);
			gap: 24px;
			box-sizing: border-box;
			height: calc(100vh - 160px);
			max-height: calc(100vh - 160px);
		}
		@media (max-width: 980px) {
			main {
				grid-template-columns: 1fr;
				height: auto;
				max-height: none;
			}
		}
		section.panel {
			background: #ffffff;
			border-radius: 12px;
			box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
			padding: 18px 20px;
			display: flex;
			flex-direction: column;
			min-height: 0;
			overflow: hidden;
		}
		.panel-header,
		.preview-header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 12px;
			margin-bottom: 14px;
			flex-wrap: wrap;
		}
		.pagination-controls {
			display: flex;
			align-items: center;
			gap: 10px;
			flex-wrap: wrap;
			font-size: 0.9rem;
			color: #475569;
		}
		.pagination-controls label {
			display: flex;
			align-items: center;
			gap: 6px;
		}
		.pagination-controls select {
			padding: 4px 6px;
			border-radius: 6px;
			border: 1px solid #cbd5e1;
			background: #ffffff;
			color: #1f2937;
			font-size: 0.9rem;
		}
		.list-panel .table-wrapper {
			flex: 1;
			overflow: auto;
			border-radius: 10px;
			border: 1px solid #d9e2ec;
			background: #ffffff;
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
		button.danger {
			background: #dc2626;
		}
		button.danger:hover:not([disabled]) {
			background: #b91c1c;
		}
		.inline-button {
			font-size: 0.85rem;
			padding: 4px 10px;
		}
		th.col-select, td.col-select {
			width: 48px;
		}
		th.col-created, td.col-created,
		th.col-updated, td.col-updated {
			width: 140px;
		}
		th.col-action, td.col-action {
			width: 90px;
		}
		.preview-panel {
			min-height: 0;
		}
		.preview-actions {
			display: flex;
			gap: 10px;
			align-items: center;
			flex-wrap: wrap;
		}
		.preview-content {
			flex: 1;
			min-height: 0;
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
			font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
		}
		.empty-placeholder {
			text-align: center;
			color: #64748b;
			margin-top: 40px;
		}
	</style>
</head>
<body>
	<div id="root"></div>
	<noscript>
		<p style="padding:16px;color:#dc2626;">需要启用 JavaScript 才能使用 Web 界面。</p>
	</noscript>
	<script>
window.__APP_CONFIG__ = {{.Config}};
	</script>
	<script crossorigin src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
	<script crossorigin src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
	<script crossorigin src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
	<script type="text/babel">
{{.AppScript}}
	</script>
</body>
</html>`

const reactAppScript = `
const appConfig = window.__APP_CONFIG__ || {};
const listenLabel = appConfig.listen || "";
const timezoneLabel = appConfig.timezone || "";

const { useState, useEffect, useMemo, useCallback, useRef } = React;

const initialPreview = {
	id: "",
	title: "",
	createTime: "",
	updateTime: "",
	messages: [],
	loading: false
};

function MessageBar({ message }) {
	const className = message.error ? "error" : message.text ? "info" : "";
	return (
		<div id="message" className={className}>
			{message.text}
		</div>
	);
}

function TableRow({ item, checked, onToggle, onPreview, previewLoading }) {
	return (
		<tr onDoubleClick={() => onToggle(item.id)}>
			<td className="col-select">
				<input
					type="checkbox"
					checked={checked}
					onChange={() => onToggle(item.id)}
				/>
			</td>
			<td className="title-cell">{item.title || "(未命名对话)"}</td>
			<td className="col-created">{item.create_time}</td>
			<td className="col-updated">{item.update_time}</td>
			<td className="col-action">
				<button
					type="button"
					className="inline-button"
					onClick={() => onPreview(item.id)}
					disabled={previewLoading}
				>
					预览
				</button>
			</td>
		</tr>
	);
}

function PreviewMessages({ preview }) {
	if (preview.loading) {
		return (
			<div className="empty-placeholder">正在加载对话…</div>
		);
	}
	if (!preview.id) {
		return (
			<div className="empty-placeholder">暂未选择对话。</div>
		);
	}
	if (!preview.messages || preview.messages.length === 0) {
		return (
			<div className="empty-placeholder">这条对话没有可展示的消息。</div>
		);
	}
	return preview.messages.map((msg, index) => {
		const role = msg.role ? msg.role.toLowerCase() : "unknown";
		const label = msg.role ? msg.role.toUpperCase() : "UNKNOWN";
		return (
			<div className={"message role-" + role} key={preview.id + "-" + index}>
				<div className="message-header">
					{label + " · " + (msg.timestamp || "-")}
				</div>
				<pre>{msg.text || ""}</pre>
			</div>
		);
	});
}

function App() {
	const [conversations, setConversations] = useState([]);
	const [total, setTotal] = useState(0);
	const [offset, setOffset] = useState(0);
	const [limit, setLimit] = useState(20);
	const [hasMore, setHasMore] = useState(false);
	const [loading, setLoading] = useState(false);
	const [forceReload, setForceReload] = useState(false);
	const [reloadToken, setReloadToken] = useState(0);
	const [message, setMessage] = useState({ text: "", error: false });
	const [selected, setSelected] = useState(() => new Set());
	const [importLoading, setImportLoading] = useState(false);
	const [bulkDeleteLoading, setBulkDeleteLoading] = useState(false);
	const [singleDeleteLoading, setSingleDeleteLoading] = useState(false);
	const [preview, setPreview] = useState(initialPreview);
	const messageTimerRef = useRef(null);

	const selectedIds = useMemo(() => Array.from(selected), [selected]);
	const selectedCount = selectedIds.length;

	const showMessage = useCallback((text, isError) => {
		if (messageTimerRef.current) {
			clearTimeout(messageTimerRef.current);
			messageTimerRef.current = null;
		}
		setMessage({ text: text || "", error: !!isError });
		if (text && !isError) {
			messageTimerRef.current = setTimeout(() => {
				setMessage({ text: "", error: false });
				messageTimerRef.current = null;
			}, 3200);
		}
	}, []);

	useEffect(() => {
		return () => {
			if (messageTimerRef.current) {
				clearTimeout(messageTimerRef.current);
			}
		};
	}, []);

	useEffect(() => {
		let cancelled = false;
		async function loadData() {
			setLoading(true);
			const params = new URLSearchParams();
			params.set("offset", String(offset));
			params.set("limit", String(limit));
			if (forceReload) {
				params.set("refresh", "1");
			}
			const pageNumber = limit > 0 ? Math.floor(offset / limit) + 1 : 1;
			showMessage(forceReload ? "正在刷新对话列表…" : "正在加载第 " + pageNumber + " 页…", false);
			try {
				const response = await fetch("/api/conversations?" + params.toString(), {
					headers: { "Accept": "application/json" }
				});
				const data = await response.json().catch(() => ({}));
				if (!response.ok) {
					throw new Error(data.error || response.statusText);
				}
				if (cancelled) {
					return;
				}
				const items = Array.isArray(data.items) ? data.items : [];
				const totalValue = typeof data.total === "number" && data.total >= 0 ? data.total : items.length;
				const offsetValue = typeof data.offset === "number" && data.offset >= 0 ? data.offset : offset;
				const limitValue = typeof data.limit === "number" && data.limit > 0 ? data.limit : limit;
				setConversations(items);
				setTotal(totalValue);
				setHasMore(Boolean(data.has_more));
				if (offsetValue !== offset) {
					setOffset(offsetValue);
				}
				if (limitValue !== limit) {
					setLimit(limitValue);
				}
				showMessage(totalValue === 0 ? "暂无对话记录" : "已加载第 " + (limitValue > 0 ? Math.floor(offsetValue / limitValue) + 1 : 1) + " 页，共 " + totalValue + " 条", false);
			} catch (error) {
				if (!cancelled) {
					showMessage(error.message || "加载列表失败", true);
				}
			} finally {
				if (!cancelled) {
					setLoading(false);
					setForceReload(false);
				}
			}
		}
		loadData();
		return () => {
			cancelled = true;
		};
	}, [offset, limit, forceReload, reloadToken, showMessage]);

	const toggleSelection = useCallback((id) => {
		setSelected((prev) => {
			const next = new Set(prev);
			if (next.has(id)) {
				next.delete(id);
			} else {
				next.add(id);
			}
			return next;
		});
	}, []);

	const clearSelection = useCallback((ids) => {
		if (!ids || ids.length === 0) {
			return;
		}
		setSelected((prev) => {
			const next = new Set(prev);
			ids.forEach((id) => next.delete(id));
			return next;
		});
	}, []);

	const handlePreview = useCallback(async (id) => {
		if (!id) {
			return;
		}
		setPreview({
			...initialPreview,
			id,
			loading: true
		});
		showMessage("正在加载对话预览…", false);
		try {
			const response = await fetch("/api/conversations/" + encodeURIComponent(id), {
				headers: { "Accept": "application/json" }
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText);
			}
			setPreview({
				id: data.id || id,
				title: data.title || data.id || "",
				createTime: data.create_time || "-",
				updateTime: data.update_time || "-",
				messages: Array.isArray(data.messages) ? data.messages : [],
				loading: false
			});
			showMessage("预览已更新", false);
		} catch (error) {
			setPreview(initialPreview);
			showMessage(error.message || "加载对话详情失败", true);
		}
	}, [showMessage]);

	const handleImport = useCallback(async () => {
		if (selectedCount === 0) {
			showMessage("请先在列表中勾选需要导入的对话", true);
			return;
		}
		setImportLoading(true);
		showMessage("正在导入 " + selectedCount + " 条对话…", false);
		try {
			const response = await fetch("/api/import", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					"Accept": "application/json"
				},
				body: JSON.stringify({ ids: selectedIds })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText);
			}
			const created = typeof data.created === "number" ? data.created : 0;
			const skipped = Array.isArray(data.skipped) ? data.skipped.length : 0;
			let text = "成功导入 " + created + " 条对话";
			if (skipped > 0) {
				text += "，跳过 " + skipped + " 条";
			}
			showMessage(text, false);
		} catch (error) {
			showMessage(error.message || "导入失败", true);
		} finally {
			setImportLoading(false);
		}
	}, [selectedCount, selectedIds, showMessage]);

	const adjustAfterDelete = useCallback((deletedIds, deletedCount, clearPreviewFlag) => {
		const count = deletedCount >= 0 ? deletedCount : deletedIds.length;
		clearSelection(deletedIds);
		if (clearPreviewFlag || (preview.id && deletedIds.indexOf(preview.id) !== -1)) {
			setPreview(initialPreview);
		}
		const newTotal = Math.max(0, total - count);
		setTotal(newTotal);
		setOffset((prevOffset) => {
			if (newTotal === 0) {
				return 0;
			}
			const maxOffset = Math.max(0, Math.floor((newTotal - 1) / limit) * limit);
			return prevOffset > maxOffset ? maxOffset : prevOffset;
		});
		setForceReload(true);
		setReloadToken((token) => token + 1);
	}, [clearSelection, total, limit, preview]);

	const performDelete = useCallback(async (ids, type) => {
		if (!ids || ids.length === 0) {
			showMessage("没有有效的对话可删除", true);
			return;
		}
		if (type === "bulk") {
			setBulkDeleteLoading(true);
		} else {
			setSingleDeleteLoading(true);
		}
		showMessage("正在删除对话…", false);
		try {
			const response = await fetch("/api/conversations/delete", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					"Accept": "application/json"
				},
				body: JSON.stringify({ ids })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText);
			}
			const deletedIds = Array.isArray(data.deleted) ? data.deleted : ids;
			const count = typeof data.count === "number" ? data.count : deletedIds.length;
			adjustAfterDelete(deletedIds, count, type === "single");
			showMessage("删除成功 " + count + " 条对话", false);
		} catch (error) {
			showMessage(error.message || "删除失败", true);
		} finally {
			if (type === "bulk") {
				setBulkDeleteLoading(false);
			} else {
				setSingleDeleteLoading(false);
			}
		}
	}, [adjustAfterDelete, showMessage]);

	const handleBulkDelete = useCallback(() => {
		if (selectedCount === 0) {
			showMessage("请先勾选需要删除的对话", true);
			return;
		}
		if (window.confirm("确认删除选中的 " + selectedCount + " 条对话吗？")) {
			performDelete(selectedIds, "bulk");
		}
	}, [performDelete, selectedCount, selectedIds, showMessage]);

	const handleSingleDelete = useCallback(() => {
		if (!preview.id) {
			showMessage("请先在左侧选择需要删除的对话", true);
			return;
		}
		if (window.confirm("确认删除当前预览的对话吗？")) {
			performDelete([preview.id], "single");
		}
	}, [performDelete, preview, showMessage]);

	const handleReload = useCallback(() => {
		setForceReload(true);
		setReloadToken((token) => token + 1);
	}, []);

	const handlePrevPage = useCallback(() => {
		setOffset((prev) => Math.max(0, prev - limit));
	}, [limit]);

	const handleNextPage = useCallback(() => {
		setOffset((prev) => prev + limit);
	}, [limit]);

	const handlePageSizeChange = useCallback((event) => {
		const value = parseInt(event.target.value, 10);
		if (!Number.isFinite(value) || value <= 0) {
			return;
		}
		setLimit(value);
		setOffset(0);
		setForceReload(true);
		setReloadToken((token) => token + 1);
	}, []);

	const totalPages = useMemo(() => {
		if (total <= 0 || limit <= 0) {
			return 0;
		}
		return Math.max(1, Math.ceil(total / limit));
	}, [total, limit]);

	const currentPage = useMemo(() => {
		if (totalPages === 0 || limit <= 0) {
			return 0;
		}
		return Math.floor(offset / limit) + 1;
	}, [offset, limit, totalPages]);

	const pageInfoText = totalPages > 0 ? "第 " + currentPage + " / " + totalPages + " 页" : loading ? "正在加载…" : "暂无对话";
	const totalLabel = "(" + total + ")";
	const importLabel = importLoading ? "导入中…" : selectedCount > 0 ? "导入所选 (" + selectedCount + ")" : "导入所选";
	const bulkDeleteLabel = bulkDeleteLoading ? "删除中…" : selectedCount > 0 ? "删除所选 (" + selectedCount + ")" : "删除所选";
	const singleDeleteLabel = singleDeleteLoading ? "删除中…" : "删除该对话";
	const canPrev = !loading && offset > 0;
	const canNext = !loading && ((hasMore && limit > 0) || (total > 0 && limit > 0 && offset + limit < total));

	return (
		<React.Fragment>
			<header>
				<h1>ChatGPT 对话导出</h1>
				<div className="meta">
					<span>监听地址: {listenLabel}</span>
					<span>输出时区: {timezoneLabel || "-"}</span>
					<div className="controls">
						<button type="button" onClick={handleReload} disabled={loading}>刷新列表</button>
						<button type="button" onClick={handleImport} disabled={selectedCount === 0 || importLoading}>{importLabel}</button>
						<button type="button" className="danger" onClick={handleBulkDelete} disabled={selectedCount === 0 || bulkDeleteLoading}>{bulkDeleteLabel}</button>
					</div>
				</div>
				<MessageBar message={message} />
			</header>
			<main>
				<section className="panel list-panel">
					<div className="panel-header">
						<h2>对话列表 <span>{totalLabel}</span></h2>
						<div className="pagination-controls">
							<button type="button" className="inline-button" onClick={handlePrevPage} disabled={!canPrev}>上一页</button>
							<span>{pageInfoText}</span>
							<button type="button" className="inline-button" onClick={handleNextPage} disabled={!canNext}>下一页</button>
							<label className="page-size">
								每页
								<select value={limit} onChange={handlePageSizeChange}>
									<option value="10">10</option>
									<option value="20">20</option>
									<option value="50">50</option>
								</select>
							</label>
						</div>
					</div>
					<div className="table-wrapper">
						<table>
							<thead>
								<tr>
									<th className="col-select">选择</th>
									<th>标题</th>
									<th className="col-created">创建时间</th>
									<th className="col-updated">最近更新</th>
									<th className="col-action">操作</th>
								</tr>
							</thead>
							<tbody>
								{loading ? (
									<tr>
										<td colSpan="5">
											<div className="empty-placeholder">正在加载…</div>
										</td>
									</tr>
								) : conversations.length === 0 ? (
									<tr>
										<td colSpan="5">
											<div className="empty-placeholder">暂未获取到对话记录</div>
										</td>
									</tr>
								) : (
									conversations.map((item) => (
										<TableRow
											key={item.id}
											item={item}
											checked={selected.has(item.id)}
											onToggle={toggleSelection}
											onPreview={handlePreview}
											previewLoading={preview.loading && preview.id === item.id}
										/>
									))
								)}
							</tbody>
						</table>
					</div>
				</section>
				<section className="panel preview-panel">
					<div className="preview-header">
						<h2>对话预览</h2>
						<div className="preview-actions">
							<button type="button" className="inline-button danger" onClick={handleSingleDelete} disabled={!preview.id || singleDeleteLoading}>{singleDeleteLabel}</button>
						</div>
					</div>
					<div className="preview-content">
						<h3>{preview.id ? (preview.title || preview.id) : "请选择左侧的对话查看详情"}</h3>
						<div className="preview-meta">
							{preview.id ? ("ID: " + preview.id + " · 创建: " + (preview.createTime || "-") + " · 最近更新: " + (preview.updateTime || "-")) : ""}
						</div>
						<div id="preview-messages">
							<PreviewMessages preview={preview} />
						</div>
					</div>
				</section>
			</main>
		</React.Fragment>
	);
}

const root = ReactDOM.createRoot(document.getElementById("root"));
root.render(<App />);
`

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
	mux.HandleFunc("/", s.serveIndex)
	mux.HandleFunc("/api/conversations", s.handleConversationList)
	mux.HandleFunc("/api/conversations/delete", s.handleDelete)
	mux.HandleFunc("/api/conversations/", s.handleConversationDetail)
	mux.HandleFunc("/api/import", s.handleImport)
	return mux
}

func (s *webServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	cfgPayload := map[string]string{
		"listen":   strings.TrimSpace(s.cfg.ServeAddr),
		"timezone": strings.TrimSpace(s.cfg.OutputTimezone),
	}
	configJSON, err := json.Marshal(cfgPayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("生成配置失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	data := struct {
		Config    template.JS
		AppScript template.JS
	}{
		Config:    template.JS(configJSON),
		AppScript: template.JS(reactAppScript),
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
