import React, { useState, useEffect, useMemo, useCallback, useRef } from "react";

const initialConfig = {
        listen: "",
        timezone: "",
        target: "anytype"
};

const initialPreview = {
        id: "",
        title: "",
        createTime: "",
        updateTime: "",
        messages: [],
        loading: false
};

function normalizeTarget(value) {
        const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
        return lower === "notion" ? "notion" : "anytype";
}

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
	const [config, setConfig] = useState(initialConfig);
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
        const [target, setTarget] = useState(initialConfig.target);
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
		async function loadConfig() {
			try {
				const response = await fetch("/api/config", {
					headers: { "Accept": "application/json" }
				});
				const data = await response.json().catch(() => ({}));
				if (cancelled) {
					return;
				}
                                const listen = typeof data.listen === "string" ? data.listen : "";
                                const timezone = typeof data.timezone === "string" ? data.timezone : "";
                                const targetValue = normalizeTarget(data.target);
                                setConfig({
                                        listen: listen,
                                        timezone: timezone,
                                        target: targetValue
                                });
                                setTarget(targetValue);
			} catch (error) {
				if (!cancelled) {
					showMessage((error && error.message) || "加载配置失败", true);
				}
			}
		}
		loadConfig();
		return () => {
			cancelled = true;
		};
	}, [showMessage]);

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
                const targetLabelForMessage = target === "notion" ? "Notion" : "Anytype";
                showMessage("正在导入 " + selectedCount + " 条对话到 " + targetLabelForMessage + "…", false);
                try {
                        const response = await fetch("/api/import", {
                                method: "POST",
                                headers: {
                                        "Content-Type": "application/json",
                                        "Accept": "application/json"
                                },
                                body: JSON.stringify({ ids: selectedIds, target })
                        });
                        const data = await response.json().catch(() => ({}));
                        if (!response.ok) {
                                throw new Error(data.error || response.statusText);
                        }
                        const created = typeof data.created === "number" ? data.created : 0;
                        const skipped = Array.isArray(data.skipped) ? data.skipped.length : 0;
                        const responseTarget = normalizeTarget(data.target);
                        const responseLabel = responseTarget === "notion" ? "Notion" : "Anytype";
                        let text = "成功导入 " + created + " 条对话到 " + responseLabel;
                        if (skipped > 0) {
                                text += "，跳过 " + skipped + " 条";
                        }
                        if (responseTarget === "notion" && Array.isArray(data.pages) && data.pages.length > 0) {
                                const sample = data.pages.slice(0, 3).join(", ");
                                text += "，Notion 页面: " + sample;
                                if (data.pages.length > 3) {
                                        text += " 等";
                                }
                        }
                        showMessage(text, false);
                } catch (error) {
                        showMessage(error.message || "导入失败", true);
                } finally {
                        setImportLoading(false);
                }
        }, [selectedCount, selectedIds, target, showMessage]);

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
        const listenLabel = config.listen || "";
        const timezoneLabel = config.timezone || "";
        const totalLabel = "(" + total + ")";
        const targetLabel = target === "notion" ? "Notion" : "Anytype";
        const importLabel = importLoading
                ? "导入中…"
                : selectedCount > 0
                        ? "导入所选到 " + targetLabel + " (" + selectedCount + ")"
                        : "导入所选到 " + targetLabel;
        const bulkDeleteLabel = bulkDeleteLoading ? "删除中…" : selectedCount > 0 ? "删除所选 (" + selectedCount + ")" : "删除所选";
        const singleDeleteLabel = singleDeleteLoading ? "删除中…" : "删除该对话";
        const canPrev = !loading && offset > 0;
        const canNext = !loading && ((hasMore && limit > 0) || (total > 0 && limit > 0 && offset + limit < total));
        const targetHint = target === "notion"
                ? "将对话同步到 Notion，请确保已在后端配置 Notion API 参数。"
                : "将对话同步到 Anytype 空间。";

        const handleTargetChange = useCallback((event) => {
                const nextTarget = normalizeTarget(event.target.value);
                setTarget(nextTarget);
                setConfig((prev) => ({ ...prev, target: nextTarget }));
        }, []);

	return (
		<React.Fragment>
			<header>
				<h1>ChatGPT 对话导出</h1>
				<div className="meta">
					<span>监听地址: {listenLabel}</span>
					<span>输出时区: {timezoneLabel || "-"}</span>
                                        <div className="controls">
                                                <label className="inline-select">
                                                        导出目标
                                                        <select value={target} onChange={handleTargetChange}>
                                                                <option value="anytype">Anytype</option>
                                                                <option value="notion">Notion</option>
                                                        </select>
                                                </label>
                                                <button type="button" onClick={handleReload} disabled={loading}>刷新列表</button>
                                                <button type="button" onClick={handleImport} disabled={selectedCount === 0 || importLoading}>{importLabel}</button>
                                                <button type="button" className="danger" onClick={handleBulkDelete} disabled={selectedCount === 0 || bulkDeleteLoading}>{bulkDeleteLabel}</button>
                                        </div>
                                        <div className="target-hint">{targetHint}</div>
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

export default App;
