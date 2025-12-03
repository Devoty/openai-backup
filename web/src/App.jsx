import React, { useState, useEffect, useMemo, useCallback, useRef } from "react";
import ConfigForm from "./components/ConfigForm";
import ConversationRow from "./components/ConversationRow";
import MessageBar from "./components/MessageBar";
import PreviewMessages from "./components/PreviewMessages";
import { configSections, initialConfig, initialPreview } from "./config/constants";
import { clampPageSizeValue, createConfigDraft, normalizeConfigResponse, normalizeTarget, prepareConfigPayload } from "./utils/config";

function App() {
	const [conversations, setConversations] = useState([]);
	const [config, setConfig] = useState(initialConfig);
	const [activeTab, setActiveTab] = useState("conversations");
	const [configTab, setConfigTab] = useState("core");
	const [configDraft, setConfigDraft] = useState(() => createConfigDraft(initialConfig));
	const [configSaving, setConfigSaving] = useState(false);
	const [total, setTotal] = useState(0);
	const [offset, setOffset] = useState(0);
	const [limit, setLimit] = useState(() => clampPageSizeValue(initialConfig.page_size));
	const [hasMore, setHasMore] = useState(false);
	const [loading, setLoading] = useState(false);
	const [forceReload, setForceReload] = useState(false);
	const [reloadToken, setReloadToken] = useState(0);
	const [message, setMessage] = useState({ text: "", error: false });
	const [selected, setSelected] = useState(() => new Set());
	const [importLoading, setImportLoading] = useState(false);
	const [exportZipLoading, setExportZipLoading] = useState(false);
	const [bulkDeleteLoading, setBulkDeleteLoading] = useState(false);
	const [singleDeleteLoading, setSingleDeleteLoading] = useState(false);
	const [preview, setPreview] = useState(initialPreview);
	const [target, setTarget] = useState(initialConfig.target);
	const [searchTerm, setSearchTerm] = useState("");
	const [configImporting, setConfigImporting] = useState(false);
	const [configExporting, setConfigExporting] = useState(false);
	const messageTimerRef = useRef(null);
	const configImportInputRef = useRef(null);

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

	const applyConfigPayloadToState = useCallback((data) => {
		const normalized = normalizeConfigResponse(data);
		setConfig(normalized);
		setTarget(normalized.target);
		setConfigDraft(createConfigDraft(normalized));
		setLimit(clampPageSizeValue(normalized.page_size));
		setOffset(0);
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
					headers: { Accept: "application/json" }
				});
				const data = await response.json().catch(() => ({}));
				if (!response.ok) {
					throw new Error(data.error || response.statusText || "加载配置失败");
				}
				if (!cancelled) {
					applyConfigPayloadToState(data);
				}
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
	}, [applyConfigPayloadToState, showMessage]);

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
					headers: { Accept: "application/json" }
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
				const limitValue = clampPageSizeValue(typeof data.limit === "number" ? data.limit : limit);
				setConversations(items);
				setTotal(totalValue);
				setHasMore(Boolean(data.has_more));
				if (offsetValue !== offset) {
					setOffset(offsetValue);
				}
				if (limitValue !== limit) {
					setLimit(limitValue);
				}
				const humanPage = limitValue > 0 ? Math.floor(offsetValue / limitValue) + 1 : 1;
				showMessage(totalValue === 0 ? "暂无对话记录" : "已加载第 " + humanPage + " 页，共 " + totalValue + " 条", false);
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

	const handleConfigFieldChange = useCallback((key, value) => {
		setConfigDraft((prev) => ({ ...prev, [key]: value }));
	}, []);

	const handleConfigReset = useCallback(() => {
		setConfigDraft(createConfigDraft(config));
	}, [config]);

	const handleOpenSettings = useCallback(() => {
		setConfigDraft(createConfigDraft(config));
		setConfigTab("core");
		setActiveTab("settings");
	}, [config]);

	const handleConfigSectionChange = useCallback((key) => {
		if (typeof key !== "string" || key.trim() === "") {
			setConfigTab("core");
			return;
		}
		const exists = configSections.some((section) => section.key === key);
		setConfigTab(exists ? key : "core");
	}, []);

	const handleConfigExport = useCallback(async () => {
		setConfigExporting(true);
		try {
			const response = await fetch("/api/config/export", {
				headers: { Accept: "application/json" }
			});
			if (!response.ok) {
				let messageText = response.statusText || "导出配置失败";
				try {
					const data = await response.json();
					if (data && data.error) {
						messageText = data.error;
					}
				} catch {
					try {
						const text = await response.text();
						if (text) {
							messageText = text;
						}
					} catch {
						// ignore secondary failure
					}
				}
				throw new Error(messageText);
			}
			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			let filename = "openai-backup-config.json";
			const disposition = response.headers.get("Content-Disposition");
			if (disposition) {
				const match = disposition.match(/filename\*=UTF-8''([^;]+)|filename=\"?([^\";]+)\"?/i);
				if (match) {
					const rawName = match[1] || match[2];
					if (rawName) {
						try {
							filename = decodeURIComponent(rawName);
						} catch {
							filename = rawName;
						}
					}
				}
			} else {
				const stamp = new Date().toISOString().replace(/[-:]/g, "").replace(/\..+/, "");
				filename = "openai-backup-config-" + stamp + ".json";
			}
			const link = document.createElement("a");
			link.href = url;
			link.download = filename;
			document.body.appendChild(link);
			link.click();
			document.body.removeChild(link);
			URL.revokeObjectURL(url);
			showMessage("配置已导出", false);
		} catch (error) {
			showMessage((error && error.message) || "导出配置失败", true);
		} finally {
			setConfigExporting(false);
		}
	}, [showMessage]);

	const handleConfigImportClick = useCallback(() => {
		if (configImportInputRef.current) {
			configImportInputRef.current.value = "";
			configImportInputRef.current.click();
		}
	}, []);

	const handleConfigImportFile = useCallback(
		(event) => {
			const input = event && event.target;
			const files = input && input.files;
			if (!files || files.length === 0) {
				return;
			}
			const file = files[0];
			if (!file) {
				return;
			}
			const reader = new FileReader();
			reader.onload = async () => {
				try {
					const text = typeof reader.result === "string" ? reader.result : "";
					if (!text) {
						throw new Error("导入文件为空");
					}
					let parsed;
					try {
						parsed = JSON.parse(text);
					} catch {
						throw new Error("导入文件不是有效的 JSON 格式");
					}
					if (!parsed || typeof parsed !== "object") {
						throw new Error("导入文件缺少配置数据");
					}
					setConfigImporting(true);
					const response = await fetch("/api/config/import", {
						method: "POST",
						headers: {
							"Content-Type": "application/json",
							Accept: "application/json"
						},
						body: JSON.stringify(parsed)
					});
					const data = await response.json().catch(() => ({}));
					if (!response.ok) {
						throw new Error(data.error || response.statusText || "导入配置失败");
					}
					applyConfigPayloadToState(data);
					showMessage("配置已导入", false);
				} catch (error) {
					showMessage((error && error.message) || "导入配置失败", true);
				} finally {
					setConfigImporting(false);
					if (configImportInputRef.current) {
						configImportInputRef.current.value = "";
					}
				}
			};
			reader.onerror = () => {
				showMessage("读取导入文件失败", true);
				if (configImportInputRef.current) {
					configImportInputRef.current.value = "";
				}
			};
			reader.readAsText(file);
		},
		[applyConfigPayloadToState, showMessage]
	);

	const handleConfigSubmit = useCallback(
		async (event) => {
			event.preventDefault();
			setConfigSaving(true);
			try {
				const payload = prepareConfigPayload(configDraft);
				const response = await fetch("/api/config", {
					method: "POST",
					headers: {
						"Content-Type": "application/json",
						Accept: "application/json"
					},
					body: JSON.stringify(payload)
				});
				const data = await response.json().catch(() => ({}));
				if (!response.ok) {
					throw new Error(data.error || response.statusText || "保存配置失败");
				}
				const normalized = normalizeConfigResponse(data);
				setConfig(normalized);
				setConfigDraft(createConfigDraft(normalized));
				setTarget(normalized.target);
				setLimit(clampPageSizeValue(normalized.page_size));
				setOffset(0);
				setSelected(new Set());
				setPreview(initialPreview);
				setForceReload(true);
				setReloadToken((token) => token + 1);
				setActiveTab("conversations");
				showMessage("配置已保存", false);
			} catch (error) {
				showMessage((error && error.message) || "保存配置失败", true);
			} finally {
				setConfigSaving(false);
			}
		},
		[configDraft, showMessage]
	);

	const handlePreview = useCallback(
		async (id) => {
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
					headers: { Accept: "application/json" }
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
		},
		[showMessage]
	);

	const handleImport = useCallback(async (targetOverride) => {
		if (selectedCount === 0) {
			showMessage("请先在列表中勾选需要导入的对话", true);
			return;
		}
		const resolvedTarget = normalizeTarget(targetOverride || target);
		setTarget(resolvedTarget);
		setImportLoading(true);
		const targetLabelForMessage = resolvedTarget === "notion" ? "Notion" : "Anytype";
		showMessage("正在导入 " + selectedCount + " 条对话到 " + targetLabelForMessage + "…", false);
		try {
			const response = await fetch("/api/import", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					Accept: "application/json"
				},
				body: JSON.stringify({ ids: selectedIds, target: resolvedTarget })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText);
			}
			const created = typeof data.created === "number" ? data.created : 0;
			const skipped = Array.isArray(data.skipped) ? data.skipped.length : 0;
			const responseTarget = normalizeTarget(data.target || resolvedTarget);
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

	const handleExportZip = useCallback(async () => {
		if (selectedCount === 0) {
			showMessage("请先在列表中勾选需要导出的对话", true);
			return;
		}
		setExportZipLoading(true);
		showMessage("正在打包选中的对话为 Markdown…", false);
		try {
			const response = await fetch("/api/conversations/export", {
				method: "POST",
				headers: {
					"Content-Type": "application/json"
				},
				body: JSON.stringify({ ids: selectedIds })
			});
			if (!response.ok) {
				let messageText = response.statusText || "导出失败";
				try {
					const data = await response.json();
					if (data && data.error) {
						messageText = data.error;
					}
				} catch {
					try {
						const text = await response.text();
						if (text) {
							messageText = text;
						}
					} catch {
						// ignore secondary failure
					}
				}
				throw new Error(messageText);
			}
			const blob = await response.blob();
			let filename = "conversations.zip";
			const disposition = response.headers.get("Content-Disposition");
			if (disposition) {
				const match = disposition.match(/filename\*=UTF-8''([^;]+)|filename=\"?([^\";]+)\"?/i);
				if (match) {
					const rawName = match[1] || match[2];
					if (rawName) {
						try {
							filename = decodeURIComponent(rawName);
						} catch {
							filename = rawName;
						}
					}
				}
			} else {
				const stamp = new Date().toISOString().replace(/[-:]/g, "").replace(/\..+/, "");
				filename = "conversations-" + stamp + ".zip";
			}
			const url = URL.createObjectURL(blob);
			const link = document.createElement("a");
			link.href = url;
			link.download = filename;
			document.body.appendChild(link);
			link.click();
			document.body.removeChild(link);
			URL.revokeObjectURL(url);
			showMessage("Markdown 压缩包已开始下载", false);
		} catch (error) {
			showMessage((error && error.message) || "导出失败", true);
		} finally {
			setExportZipLoading(false);
		}
	}, [selectedCount, selectedIds, showMessage]);

	const adjustAfterDelete = useCallback(
		(deletedIds, deletedCount, clearPreviewFlag) => {
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
		},
		[clearSelection, total, limit, preview]
	);

	const performDelete = useCallback(
		async (ids, type) => {
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
						Accept: "application/json"
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
		},
		[adjustAfterDelete, showMessage]
	);

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
	const exportZipLabel =
		exportZipLoading
			? "打包中…"
			: selectedCount > 0
				? "导出所选为 Markdown (" + selectedCount + ")"
				: "导出所选为 Markdown";
	const bulkDeleteLabel = bulkDeleteLoading ? "删除中…" : selectedCount > 0 ? "删除所选 (" + selectedCount + ")" : "删除所选";
	const singleDeleteLabel = singleDeleteLoading ? "删除中…" : "删除该对话";
	const canPrev = !loading && offset > 0;
	const canNext = !loading && ((hasMore && limit > 0) || (total > 0 && limit > 0 && offset + limit < total));
	const targetHint = target === "notion" ? "将对话同步到 Notion，请确保已在后端配置 Notion API 参数。" : "将对话同步到 Anytype 空间。";

	const handleTargetChange = useCallback((event) => {
		const nextTarget = normalizeTarget(event.target.value);
		setTarget(nextTarget);
		setConfig((prev) => ({ ...prev, target: nextTarget }));
	}, []);

	const handleSearchChange = useCallback((event) => {
		setSearchTerm(event.target.value || "");
	}, []);

	const filteredConversations = useMemo(() => {
		const term = (searchTerm || "").trim().toLowerCase();
		if (!term) {
			return conversations;
		}
		return conversations.filter((item) => {
			const title = (item.title || "").toLowerCase();
			const id = (item.id || "").toLowerCase();
			const createTime = (item.create_time || "").toLowerCase();
			const updateTime = (item.update_time || "").toLowerCase();
			return title.includes(term) || id.includes(term) || createTime.includes(term) || updateTime.includes(term);
		});
	}, [conversations, searchTerm]);

	const handleBackToList = useCallback(() => {
		setPreview(initialPreview);
	}, []);

	return (
		<React.Fragment>
			<header className="app-header">
				<div className="brand">
					<div className="brand-logo">B</div>
					<div className="brand-text">
						<div className="brand-title">Backed</div>
						<div className="brand-subtitle">ChatGPT 对话整理与导出</div>
					</div>
				</div>
				<div className="brand-meta">
					<div className="pill">监听 {listenLabel || "-"}</div>
					<div className="pill">时区 {timezoneLabel || "-"}</div>
					<button
						type="button"
						className={`ghost ${activeTab === "settings" ? "active" : ""}`}
						onClick={handleOpenSettings}
					>
						设置
					</button>
				</div>
			</header>
			<MessageBar message={message} />
			{activeTab === "settings" ? (
				<div className="settings-container">
					<div className="settings-meta-banner">分区配置导出路径与 Token，保存后立即生效。</div>
					<ConfigForm
						draft={configDraft}
						sections={configSections}
						onFieldChange={handleConfigFieldChange}
						onSubmit={handleConfigSubmit}
						onReset={handleConfigReset}
						saving={configSaving}
						activeSection={configTab}
						onSectionChange={handleConfigSectionChange}
						onImport={handleConfigImportClick}
						onExport={handleConfigExport}
						importing={configImporting}
						exporting={configExporting}
					/>
					<input ref={configImportInputRef} type="file" accept="application/json" style={{ display: "none" }} onChange={handleConfigImportFile} />
				</div>
			) : (
				<div className="workspace">
					<div className="global-toolbar">
						<div className="toolbar-left">
							<div className="toolbar-title">对话管理</div>
							<div className="toolbar-subtitle">
								<span>共 {total} 条</span>
								<span className="dot">•</span>
								<span>{pageInfoText}</span>
							</div>
						</div>
						<div className="toolbar-actions">
							<div className="button-group">
								<button type="button" onClick={handleExportZip} disabled={selectedCount === 0 || exportZipLoading}>
									{exportZipLabel}
								</button>
								<button
									type="button"
									className="secondary"
									onClick={() => handleImport("notion")}
									disabled={selectedCount === 0 || importLoading}
								>
									导出到 Notion
								</button>
								<button
									type="button"
									className="secondary"
									onClick={() => handleImport("anytype")}
									disabled={selectedCount === 0 || importLoading}
								>
									导出到 Anytype
								</button>
							</div>
							<div className="button-group ghost-group">
								<button type="button" className="ghost" onClick={handleReload} disabled={loading}>
									刷新
								</button>
								<button type="button" className="ghost danger-outline" onClick={handleBulkDelete} disabled={selectedCount === 0 || bulkDeleteLoading}>
									{bulkDeleteLabel}
								</button>
							</div>
						</div>
					</div>
					<main className="content-grid">
						<section className="panel list-panel">
							<div className="panel-header list-panel-header">
								<div className="list-heading">
									<h2>
										对话列表 <span>{totalLabel}</span>
									</h2>
									<div className="target-hint muted">{targetHint}</div>
								</div>
								<div className="list-tools">
									<div className="search-box">
										<input type="search" value={searchTerm} onChange={handleSearchChange} placeholder="搜索标题 / ID / 时间" />
									</div>
									<label className="inline-select">
										导出目标
										<select value={target} onChange={handleTargetChange}>
											<option value="anytype">Anytype</option>
											<option value="notion">Notion</option>
										</select>
									</label>
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
							<div className="list-body">
								{loading ? (
									<div className="empty-placeholder">正在加载…</div>
								) : filteredConversations.length === 0 ? (
									<div className="empty-placeholder">{searchTerm ? "没有匹配的对话" : "暂未获取到对话记录"}</div>
								) : (
									<div className="conversation-list">
										{filteredConversations.map((item) => (
											<ConversationRow
												key={item.id}
												item={item}
												checked={selected.has(item.id)}
												active={preview.id === item.id}
												onToggle={toggleSelection}
												onPreview={handlePreview}
												previewLoading={preview.loading && preview.id === item.id}
											/>
										))}
									</div>
								)}
							</div>
							<div className="pagination-bar">
								<button type="button" className="ghost" onClick={handlePrevPage} disabled={!canPrev}>
									上一页
								</button>
								<div className="page-info">{pageInfoText}</div>
								<button type="button" className="ghost" onClick={handleNextPage} disabled={!canNext}>
									下一页
								</button>
							</div>
						</section>
						<section className="panel preview-panel">
							<div className="preview-header">
								<div className="preview-title-group">
									<button type="button" className="ghost" onClick={handleBackToList}>
										返回
									</button>
									<div className="preview-title-wrap">
										<div className="preview-title">{preview.id ? preview.title || preview.id : "请选择左侧的对话查看详情"}</div>
										<div className="preview-subtitle">最近更新 {preview.updateTime || "-"}</div>
									</div>
								</div>
								<div className="preview-actions">
									<div className="button-group">
										<button type="button" className="secondary" onClick={handleExportZip} disabled={selectedCount === 0 || exportZipLoading}>
											导出 Markdown
										</button>
										<button type="button" className="secondary" onClick={() => handleImport("notion")} disabled={selectedCount === 0 || importLoading}>
											导出 Notion
										</button>
										<button type="button" className="secondary" onClick={() => handleImport("anytype")} disabled={selectedCount === 0 || importLoading}>
											导出 Anytype
										</button>
									</div>
									<button type="button" className="danger" onClick={handleSingleDelete} disabled={!preview.id || singleDeleteLoading}>
										{singleDeleteLabel}
									</button>
								</div>
							</div>
							<div className="preview-meta">
								<div className="meta-grid">
									<div className="meta-item">
										<span>对话 ID</span>
										<strong>{preview.id || "-"}</strong>
									</div>
									<div className="meta-item">
										<span>创建时间</span>
										<strong>{preview.createTime || "-"}</strong>
									</div>
									<div className="meta-item">
										<span>更新时间</span>
										<strong>{preview.updateTime || "-"}</strong>
									</div>
									<div className="meta-item">
										<span>来源</span>
										<strong>-</strong>
									</div>
								</div>
							</div>
							<div className="preview-content">
								<div className="message-wrapper">
									<PreviewMessages preview={preview} />
								</div>
							</div>
						</section>
					</main>
				</div>
			)}
		</React.Fragment>
	);
}

export default App;
