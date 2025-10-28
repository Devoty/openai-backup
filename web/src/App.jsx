import React, { useState, useEffect, useMemo, useCallback, useRef } from "react";

const initialConfig = {
        listen: "",
        timezone: "",
        target: "anytype",
        base_url: "",
        order: "updated",
        page_size: 20,
        max_conversations: 0,
        initial_offset: 0,
        include_archived: false,
        token: "",
        device_id: "",
        user_agent: "",
        accept_language: "",
        referer: "",
        cookie: "",
        origin: "",
        oai_language: "",
        sec_ch_ua: "",
        sec_ch_ua_mobile: "",
        sec_ch_ua_platform: "",
        sec_fetch_dest: "",
        sec_fetch_mode: "",
        sec_fetch_site: "",
        chatgpt_account_id: "",
        oai_client_version: "",
        priority: "",
        log_path: "",
        anytype_base_url: "",
        anytype_version: "",
        anytype_space_id: "",
        anytype_type_key: "",
        anytype_token: "",
        notion_base_url: "",
        notion_version: "",
        notion_token: "",
        notion_parent_type: "",
        notion_parent_id: "",
        notion_title_property: ""
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

function sanitizeOrder(value) {
        const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
        return lower === "created" ? "created" : "updated";
}

function sanitizeParentType(value) {
	const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
	if (lower === "page" || lower === "database") {
		return lower;
	}
	return "";
}

function toNumber(value) {
	if (typeof value === "number" && Number.isFinite(value)) {
		return value;
	}
	if (typeof value === "string" && value.trim() !== "") {
                const parsed = Number.parseInt(value.trim(), 10);
                if (!Number.isNaN(parsed)) {
                        return parsed;
                }
        }
        return undefined;
}

function clampPageSizeValue(value) {
        const parsed = toNumber(value);
        if (typeof parsed !== "number" || Number.isNaN(parsed)) {
                return 20;
        }
        if (parsed < 1) {
                return 1;
        }
        if (parsed > 100) {
                return 100;
        }
        return parsed;
}

function normalizeConfigResponse(data) {
        const normalized = { ...initialConfig };
        if (!data || typeof data !== "object") {
                return normalized;
        }
        const assignString = (key) => {
                if (typeof data[key] === "string") {
                        normalized[key] = data[key];
                }
        };
        assignString("listen");
        assignString("timezone");
        assignString("base_url");
        assignString("token");
        assignString("device_id");
        assignString("user_agent");
        assignString("accept_language");
        assignString("referer");
        assignString("cookie");
        assignString("origin");
        assignString("oai_language");
        assignString("sec_ch_ua");
        assignString("sec_ch_ua_mobile");
        assignString("sec_ch_ua_platform");
        assignString("sec_fetch_dest");
        assignString("sec_fetch_mode");
        assignString("sec_fetch_site");
        assignString("chatgpt_account_id");
        assignString("oai_client_version");
        assignString("priority");
        assignString("log_path");
        assignString("anytype_base_url");
        assignString("anytype_version");
        assignString("anytype_space_id");
        assignString("anytype_type_key");
        assignString("anytype_token");
        assignString("notion_base_url");
        assignString("notion_version");
        assignString("notion_token");
        assignString("notion_parent_id");
        assignString("notion_title_property");

        normalized.target = normalizeTarget(data.target);
        normalized.order = sanitizeOrder(data.order);
        normalized.page_size = clampPageSizeValue(data.page_size);

        const maxValue = toNumber(data.max_conversations);
        normalized.max_conversations = typeof maxValue === "number" && maxValue >= 0 ? maxValue : 0;

        const offsetValue = toNumber(data.initial_offset);
        normalized.initial_offset = typeof offsetValue === "number" && offsetValue >= 0 ? offsetValue : 0;

        normalized.include_archived = Boolean(data.include_archived);
        normalized.notion_parent_type = sanitizeParentType(data.notion_parent_type);

        return normalized;
}

function createConfigDraft(config) {
        const source = config || initialConfig;
        const maxValue = toNumber(source.max_conversations);
        const offsetValue = toNumber(source.initial_offset);
        return {
                listen: source.listen || "",
                timezone: source.timezone || "",
                target: normalizeTarget(source.target),
                base_url: source.base_url || "",
                order: sanitizeOrder(source.order),
                page_size: String(clampPageSizeValue(source.page_size)),
                max_conversations: String(Math.max(0, typeof maxValue === "number" ? maxValue : 0)),
                initial_offset: String(Math.max(0, typeof offsetValue === "number" ? offsetValue : 0)),
                include_archived: !!source.include_archived,
                token: source.token || "",
                device_id: source.device_id || "",
                user_agent: source.user_agent || "",
                accept_language: source.accept_language || "",
                referer: source.referer || "",
                cookie: source.cookie || "",
                origin: source.origin || "",
                oai_language: source.oai_language || "",
                sec_ch_ua: source.sec_ch_ua || "",
                sec_ch_ua_mobile: source.sec_ch_ua_mobile || "",
                sec_ch_ua_platform: source.sec_ch_ua_platform || "",
                sec_fetch_dest: source.sec_fetch_dest || "",
                sec_fetch_mode: source.sec_fetch_mode || "",
                sec_fetch_site: source.sec_fetch_site || "",
                chatgpt_account_id: source.chatgpt_account_id || "",
                oai_client_version: source.oai_client_version || "",
                priority: source.priority || "",
                log_path: source.log_path || "",
                anytype_base_url: source.anytype_base_url || "",
                anytype_version: source.anytype_version || "",
                anytype_space_id: source.anytype_space_id || "",
                anytype_type_key: source.anytype_type_key || "",
                anytype_token: source.anytype_token || "",
                notion_base_url: source.notion_base_url || "",
                notion_version: source.notion_version || "",
                notion_token: source.notion_token || "",
                notion_parent_type: sanitizeParentType(source.notion_parent_type),
                notion_parent_id: source.notion_parent_id || "",
                notion_title_property: source.notion_title_property || ""
        };
}

function prepareConfigPayload(draft) {
        const maxValue = toNumber(draft.max_conversations);
        const offsetValue = toNumber(draft.initial_offset);
        return {
                listen: (draft.listen || "").trim(),
                timezone: (draft.timezone || "").trim(),
                target: normalizeTarget(draft.target),
                base_url: (draft.base_url || "").trim(),
                order: sanitizeOrder(draft.order),
                page_size: clampPageSizeValue(draft.page_size),
                max_conversations: Math.max(0, typeof maxValue === "number" ? maxValue : 0),
                initial_offset: Math.max(0, typeof offsetValue === "number" ? offsetValue : 0),
                include_archived: !!draft.include_archived,
                token: (draft.token || "").trim(),
                device_id: (draft.device_id || "").trim(),
                user_agent: (draft.user_agent || "").trim(),
                accept_language: (draft.accept_language || "").trim(),
                referer: (draft.referer || "").trim(),
                cookie: (draft.cookie || "").trim(),
                origin: (draft.origin || "").trim(),
                oai_language: (draft.oai_language || "").trim(),
                sec_ch_ua: (draft.sec_ch_ua || "").trim(),
                sec_ch_ua_mobile: (draft.sec_ch_ua_mobile || "").trim(),
                sec_ch_ua_platform: (draft.sec_ch_ua_platform || "").trim(),
                sec_fetch_dest: (draft.sec_fetch_dest || "").trim(),
                sec_fetch_mode: (draft.sec_fetch_mode || "").trim(),
                sec_fetch_site: (draft.sec_fetch_site || "").trim(),
                chatgpt_account_id: (draft.chatgpt_account_id || "").trim(),
                oai_client_version: (draft.oai_client_version || "").trim(),
                priority: (draft.priority || "").trim(),
                log_path: (draft.log_path || "").trim(),
                anytype_base_url: (draft.anytype_base_url || "").trim(),
                anytype_version: (draft.anytype_version || "").trim(),
                anytype_space_id: (draft.anytype_space_id || "").trim(),
                anytype_type_key: (draft.anytype_type_key || "").trim(),
                anytype_token: (draft.anytype_token || "").trim(),
                notion_base_url: (draft.notion_base_url || "").trim(),
                notion_version: (draft.notion_version || "").trim(),
                notion_token: (draft.notion_token || "").trim(),
                notion_parent_type: sanitizeParentType(draft.notion_parent_type),
                notion_parent_id: (draft.notion_parent_id || "").trim(),
                notion_title_property: (draft.notion_title_property || "").trim()
        };
}

const configSections = [
        {
                key: "core",
                title: "基础配置",
                description: "配置 Web 服务和 ChatGPT 接口的关键参数。",
                fields: [
                        { key: "listen", label: "Web 监听地址", placeholder: "127.0.0.1:8080" },
                        { key: "timezone", label: "输出时区", placeholder: "Local 或 UTC" },
                        {
                                key: "target",
                                label: "默认导出目标",
                                type: "select",
                                options: [
                                        { value: "anytype", label: "Anytype" },
                                        { value: "notion", label: "Notion" }
                                ]
                        },
                        { key: "base_url", label: "ChatGPT 接口地址", placeholder: "https://chatgpt.com/backend-api" },
                        {
                                key: "order",
                                label: "对话排序",
                                type: "select",
                                options: [
                                        { value: "updated", label: "按更新时间" },
                                        { value: "created", label: "按创建时间" }
                                ]
                        },
                        {
                                key: "page_size",
                                label: "分页大小",
                                type: "number",
                                min: 1,
                                max: 100,
                                description: "每次请求的对话数量，范围 1-100。"
                        },
                        {
                                key: "max_conversations",
                                label: "最多导出数量",
                                type: "number",
                                min: 0,
                                description: "0 表示不限制。"
                        },
                        { key: "initial_offset", label: "起始 Offset", type: "number", min: 0 },
                        { key: "include_archived", label: "包含归档对话", type: "checkbox", description: "启用后会请求已归档的对话。" },
                        {
                                key: "token",
                                label: "OpenAI Bearer Token",
                                type: "textarea",
                                rows: 2,
                                fullWidth: true,
                                description: "访问 ChatGPT 接口所需的鉴权 Token。"
                        },
                        { key: "log_path", label: "日志文件路径", placeholder: "chatgpt_export.log" }
                ]
        },
        {
                key: "headers",
                title: "HTTP 请求头",
                description: "自定义与 ChatGPT 通讯时使用的请求头。",
                fields: [
                        { key: "user_agent", label: "User-Agent" },
                        { key: "device_id", label: "oai-device-id" },
                        { key: "accept_language", label: "Accept-Language" },
                        { key: "referer", label: "Referer" },
                        {
                                key: "cookie",
                                label: "Cookie",
                                type: "textarea",
                                rows: 3,
                                fullWidth: true,
                                description: "如果需要带上登录态，可以填写完整的 Cookie。"
                        },
                        { key: "origin", label: "Origin" },
                        { key: "oai_language", label: "oai-language" },
                        { key: "sec_ch_ua", label: "sec-ch-ua" },
                        { key: "sec_ch_ua_mobile", label: "sec-ch-ua-mobile" },
                        { key: "sec_ch_ua_platform", label: "sec-ch-ua-platform" },
                        { key: "sec_fetch_dest", label: "sec-fetch-dest" },
                        { key: "sec_fetch_mode", label: "sec-fetch-mode" },
                        { key: "sec_fetch_site", label: "sec-fetch-site" },
                        { key: "chatgpt_account_id", label: "chatgpt-account-id" },
                        { key: "oai_client_version", label: "oai-client-version" },
                        { key: "priority", label: "priority" }
                ]
        },
        {
                key: "anytype",
                title: "Anytype 设置",
                description: "配置 Anytype 同步所需参数。",
                fields: [
                        { key: "anytype_base_url", label: "Anytype 基础地址" },
                        { key: "anytype_version", label: "Anytype API 版本" },
                        { key: "anytype_space_id", label: "Anytype Space ID" },
                        { key: "anytype_type_key", label: "Anytype 对象类型 Key" },
                        {
                                key: "anytype_token",
                                label: "Anytype API Key",
                                type: "textarea",
                                rows: 2,
                                fullWidth: true
                        }
                ]
        },
        {
                key: "notion",
                title: "Notion 设置",
                description: "配置 Notion 导出所需参数。",
                fields: [
                        { key: "notion_base_url", label: "Notion 基础地址" },
                        { key: "notion_version", label: "Notion API 版本" },
                        {
                                key: "notion_token",
                                label: "Notion API Key",
                                type: "textarea",
                                rows: 2,
                                fullWidth: true
                        },
                        {
                                key: "notion_parent_type",
                                label: "父级类型",
                                type: "select",
                                options: [
                                        { value: "", label: "自动" },
                                        { value: "page", label: "页面 (page)" },
                                        { value: "database", label: "数据库 (database)" }
                                ]
                        },
                        { key: "notion_parent_id", label: "父级页面 / 数据库 ID" },
                        { key: "notion_title_property", label: "标题属性名称" }
                ]
        }
];

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

function ConfigField({ field, value, onChange }) {
        const { key, label, type = "text", placeholder, options = [], description, rows = 3, min, max, fullWidth } = field;
        const fieldClassName = fullWidth ? "form-field full-width" : "form-field";
        const fieldId = "config-" + key;

        if (type === "checkbox") {
                return (
                        <div className={fieldClassName}>
                                <label className="checkbox-row">
                                        <input
                                                id={fieldId}
                                                type="checkbox"
                                                checked={!!value}
                                                onChange={(event) => onChange(key, event.target.checked)}
                                        />
                                        <span>{label}</span>
                                </label>
                                {description ? <div className="field-hint">{description}</div> : null}
                        </div>
                );
        }

        if (type === "select") {
                return (
                        <div className={fieldClassName}>
                                <label htmlFor={fieldId}>{label}</label>
                                <select
                                        id={fieldId}
                                        value={value == null ? "" : value}
                                        onChange={(event) => onChange(key, event.target.value)}
                                >
                                        {options.map((option) => (
                                                <option key={option.value == null ? "" : option.value} value={option.value == null ? "" : option.value}>
                                                        {option.label}
                                                </option>
                                        ))}
                                </select>
                                {description ? <div className="field-hint">{description}</div> : null}
                        </div>
                );
        }

        if (type === "textarea") {
                return (
                        <div className={fieldClassName}>
                                <label htmlFor={fieldId}>{label}</label>
                                <textarea
                                        id={fieldId}
                                        rows={rows || 3}
                                        value={value == null ? "" : value}
                                        onChange={(event) => onChange(key, event.target.value)}
                                        placeholder={placeholder}
                                />
                                {description ? <div className="field-hint">{description}</div> : null}
                        </div>
                );
        }

        return (
                <div className={fieldClassName}>
                        <label htmlFor={fieldId}>{label}</label>
                        <input
                                id={fieldId}
                                type={type}
                                value={value == null ? "" : value}
                                onChange={(event) => onChange(key, event.target.value)}
                                placeholder={placeholder}
                                min={min}
                                max={max}
                                inputMode={type === "number" ? "numeric" : undefined}
                        />
                        {description ? <div className="field-hint">{description}</div> : null}
                </div>
        );
}

function ConfigSection({ section, draft, onFieldChange }) {
        return (
                <section className="settings-section">
                        <h2>{section.title}</h2>
                        {section.description ? <p>{section.description}</p> : null}
                        <div className="settings-grid">
                                {section.fields.map((field) => (
                                        <ConfigField
                                                key={field.key}
                                                field={field}
                                                value={draft[field.key]}
                                                onChange={onFieldChange}
                                        />
                                ))}
                        </div>
                </section>
        );
}

function ConfigForm({
        draft,
        onFieldChange,
        onSubmit,
        onReset,
        saving,
        locked,
        activeSection,
        onSectionChange,
        onImport,
        onExport,
        importing,
        exporting
}) {
        const currentSection = useMemo(() => {
                if (!configSections || configSections.length === 0) {
                        return null;
                }
                return configSections.find((section) => section.key === activeSection) || configSections[0];
        }, [activeSection]);

        return (
                <form className="settings-form" onSubmit={onSubmit}>
                        <div className="settings-tabs">
                                <div className="tab-list">
                                        {configSections.map((section) => {
                                                const isActive = currentSection ? section.key === currentSection.key : false;
                                                return (
                                                        <button
                                                                key={section.key}
                                                                type="button"
                                                                className={isActive ? "active" : ""}
                                                                onClick={() => onSectionChange ? onSectionChange(section.key) : null}
                                                        >
                                                                {section.title}
                                                        </button>
                                                );
                                        })}
                                </div>
                                <div className="tab-actions">
                                        <button
                                                type="button"
                                                className="secondary"
                                                onClick={() => onImport ? onImport() : null}
                                                disabled={locked || importing || saving}
                                        >
                                                {importing ? "导入中…" : "导入基础数据"}
                                        </button>
                                        <button
                                                type="button"
                                                onClick={() => onExport ? onExport() : null}
                                                disabled={locked || exporting}
                                        >
                                                {exporting ? "导出中…" : "导出基础数据"}
                                        </button>
                                </div>
                        </div>
                        {currentSection ? (
                                <ConfigSection section={currentSection} draft={draft} onFieldChange={onFieldChange} />
                        ) : null}
                        <div className="form-actions">
                                <button type="button" className="secondary" onClick={onReset} disabled={saving}>
                                        重置修改
                                </button>
                                <button type="submit" disabled={saving || locked}>
                                        {saving ? "保存中…" : locked ? "请先解锁" : "保存配置"}
                                </button>
                        </div>
                </form>
        );
}

function App() {
        const [conversations, setConversations] = useState([]);
        const [config, setConfig] = useState(initialConfig);
        const [activeTab, setActiveTab] = useState("conversations");
        const [configTab, setConfigTab] = useState("core");
        const [configDraft, setConfigDraft] = useState(() => createConfigDraft(initialConfig));
        const [configSaving, setConfigSaving] = useState(false);
        const [configState, setConfigState] = useState({ hasPassword: false, unlocked: true });
        const [stateVersion, setStateVersion] = useState(0);
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
        const [bulkDeleteLoading, setBulkDeleteLoading] = useState(false);
        const [singleDeleteLoading, setSingleDeleteLoading] = useState(false);
        const [preview, setPreview] = useState(initialPreview);
        const [target, setTarget] = useState(initialConfig.target);
	const [unlockPassword, setUnlockPassword] = useState("");
	const [unlockLoading, setUnlockLoading] = useState(false);
	const [passwordSaving, setPasswordSaving] = useState(false);
	const [passwordInputs, setPasswordInputs] = useState({ password: "", oldPassword: "", newPassword: "" });
        const [configImporting, setConfigImporting] = useState(false);
        const [configExporting, setConfigExporting] = useState(false);
        const messageTimerRef = useRef(null);
        const configImportInputRef = useRef(null);

	const selectedIds = useMemo(() => Array.from(selected), [selected]);
	const selectedCount = selectedIds.length;
	const isConfigLocked = configState.hasPassword && !configState.unlocked;

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

	const refreshConfigState = useCallback(() => {
		setStateVersion((token) => token + 1);
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
		async function loadStateAndConfig() {
			try {
				const response = await fetch("/api/config/state", {
					headers: { "Accept": "application/json" }
				});
				const data = await response.json().catch(() => ({}));
				if (cancelled) {
					return;
				}
				const hasPassword = Boolean(data && data.has_password);
				const unlocked = hasPassword ? Boolean(data.unlocked) : true;
				setConfigState({ hasPassword, unlocked });
				if (!unlocked) {
					return;
				}
				try {
					const cfgResponse = await fetch("/api/config", {
						headers: { "Accept": "application/json" }
					});
					const cfgData = await cfgResponse.json().catch(() => ({}));
					if (!cfgResponse.ok) {
						throw new Error(cfgData.error || cfgResponse.statusText || "加载配置失败");
					}
					if (!cancelled) {
						applyConfigPayloadToState(cfgData);
					}
				} catch (error) {
					if (!cancelled) {
						showMessage((error && error.message) || "加载配置失败", true);
					}
				}
			} catch (error) {
				if (!cancelled) {
					showMessage((error && error.message) || "加载配置状态失败", true);
				}
			}
		}
		loadStateAndConfig();
		return () => {
			cancelled = true;
		};
	}, [applyConfigPayloadToState, showMessage, stateVersion]);

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
        }, [setConfigTab]);

        const handleConfigExport = useCallback(async () => {
                if (isConfigLocked) {
                        showMessage("请先解锁配置后再导出", true);
                        return;
                }
                setConfigExporting(true);
                try {
                        const response = await fetch("/api/config/export", {
                                headers: { "Accept": "application/json" }
                        });
                        if (!response.ok) {
                                let message = response.statusText || "导出配置失败";
                                try {
                                        const data = await response.json();
                                        if (data && data.error) {
                                                message = data.error;
                                        }
                                } catch {
                                        try {
                                                const text = await response.text();
                                                if (text) {
                                                        message = text;
                                                }
                                        } catch {
                                                // ignore secondary failure
                                        }
                                }
                                throw new Error(message);
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
        }, [isConfigLocked, showMessage]);

        const handleConfigImportClick = useCallback(() => {
                if (isConfigLocked) {
                        showMessage("请先解锁配置后再导入", true);
                        return;
                }
                if (configImportInputRef.current) {
                        configImportInputRef.current.value = "";
                        configImportInputRef.current.click();
                }
        }, [isConfigLocked, showMessage, configImportInputRef]);

        const handleConfigImportFile = useCallback((event) => {
                if (isConfigLocked) {
                        showMessage("请先解锁配置后再导入", true);
                        if (event && event.target) {
                                event.target.value = "";
                        }
                        return;
                }
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
                                } catch (parseError) {
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
                                                "Accept": "application/json"
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
        }, [applyConfigPayloadToState, configImportInputRef, isConfigLocked, showMessage]);

        const handleConfigSubmit = useCallback(async (event) => {
                event.preventDefault();
		if (isConfigLocked) {
			showMessage("请先解锁配置后再保存修改", true);
			return;
		}
                setConfigSaving(true);
                try {
                        const payload = prepareConfigPayload(configDraft);
                        const response = await fetch("/api/config", {
                                method: "POST",
                                headers: {
                                        "Content-Type": "application/json",
                                        "Accept": "application/json"
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
	}, [configDraft, isConfigLocked, showMessage]);

	const handleUnlockSubmit = useCallback(async (event) => {
		event.preventDefault();
		if (!configState.hasPassword) {
			showMessage("尚未设置密码", true);
			return;
		}
		const password = (unlockPassword || "").trim();
		if (!password) {
			showMessage("请输入密码", true);
			return;
		}
		setUnlockLoading(true);
		try {
			const response = await fetch("/api/config/unlock", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					"Accept": "application/json"
				},
				body: JSON.stringify({ password })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText || "解锁失败");
			}
			setUnlockPassword("");
			setConfigState({ hasPassword: true, unlocked: true });
			if (data && typeof data === "object" && data.listen !== undefined) {
				applyConfigPayloadToState(data);
			} else {
				refreshConfigState();
			}
			showMessage("配置已解锁", false);
		} catch (error) {
			showMessage((error && error.message) || "解锁失败", true);
		} finally {
			setUnlockLoading(false);
		}
	}, [applyConfigPayloadToState, configState.hasPassword, refreshConfigState, showMessage, unlockPassword]);

	const handlePasswordInputChange = useCallback((key, value) => {
		setPasswordInputs((prev) => ({ ...prev, [key]: value }));
	}, []);

	const handleSetPassword = useCallback(async () => {
		const password = (passwordInputs.password || "").trim();
		if (password.length < 8) {
			showMessage("密码长度至少 8 位", true);
			return;
		}
		setPasswordSaving(true);
		try {
			const response = await fetch("/api/config/password", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					"Accept": "application/json"
				},
				body: JSON.stringify({ password })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText || "设置密码失败");
			}
			setConfigState({ hasPassword: true, unlocked: true });
			setPasswordInputs({ password: "", oldPassword: "", newPassword: "" });
			refreshConfigState();
			showMessage("密码已设置", false);
		} catch (error) {
			showMessage((error && error.message) || "设置密码失败", true);
		} finally {
			setPasswordSaving(false);
		}
	}, [passwordInputs.password, refreshConfigState, showMessage]);

	const handleChangePassword = useCallback(async () => {
		const oldPassword = (passwordInputs.oldPassword || "").trim();
		const newPassword = (passwordInputs.newPassword || "").trim();
		if (!oldPassword || !newPassword) {
			showMessage("请输入旧密码和新密码", true);
			return;
		}
		if (newPassword.length < 8) {
			showMessage("新密码长度至少 8 位", true);
			return;
		}
		setPasswordSaving(true);
		try {
			const response = await fetch("/api/config/password", {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
					"Accept": "application/json"
				},
				body: JSON.stringify({ old_password: oldPassword, new_password: newPassword })
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				throw new Error(data.error || response.statusText || "修改密码失败");
			}
			setPasswordInputs({ password: "", oldPassword: "", newPassword: "" });
			refreshConfigState();
			showMessage("密码已更新", false);
		} catch (error) {
			showMessage((error && error.message) || "修改密码失败", true);
		} finally {
			setPasswordSaving(false);
		}
	}, [passwordInputs.newPassword, passwordInputs.oldPassword, refreshConfigState, showMessage]);

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
		if (isConfigLocked) {
			showMessage("请先解锁配置", true);
			return;
		}
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
	}, [isConfigLocked, selectedCount, selectedIds, target, showMessage]);

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
                                <div className="header-top">
                                        <h1>ChatGPT 对话导出</h1>
                                        <nav className="app-nav">
                                                <button
                                                        type="button"
                                                        className={activeTab === "conversations" ? "active" : ""}
                                                        onClick={() => setActiveTab("conversations")}
                                                >
                                                        对话列表
                                                </button>
                                                <button
                                                        type="button"
                                                        className={activeTab === "settings" ? "active" : ""}
                                                        onClick={handleOpenSettings}
                                                >
                                                        配置管理
                                                </button>
                                        </nav>
                                </div>
                                {activeTab === "conversations" ? (
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
                                                        <button type="button" onClick={handleReload} disabled={loading}>
                                                                刷新列表
                                                        </button>
                                                        <button
                                                                type="button"
                                                                onClick={handleImport}
                                                                disabled={isConfigLocked || selectedCount === 0 || importLoading}
                                                        >
                                                                {importLabel}
                                                        </button>
                                                        <button
                                                                type="button"
                                                                className="danger"
                                                                onClick={handleBulkDelete}
                                                                disabled={selectedCount === 0 || bulkDeleteLoading}
                                                        >
                                                                {bulkDeleteLabel}
                                                        </button>
                                                </div>
                                                <div className="target-hint">{targetHint}</div>
                                        </div>
			) : (
				<div className="settings-meta">
					{configState.hasPassword
						? "在此页面修改运行及导出所需的参数，保存后立即生效。"
						: "首次使用前请设置配置密码，配置将以该密码加密存储。"}
				</div>
			)}
                                {configState.hasPassword && !configState.unlocked ? (
                                        <form className="unlock-banner" onSubmit={handleUnlockSubmit}>
                                                <input
                                                        type="password"
                                                        value={unlockPassword}
                                                        onChange={(event) => setUnlockPassword(event.target.value)}
                                                        placeholder="请输入配置密码以解锁"
                                                        disabled={unlockLoading}
                                                />
                                                <button type="submit" disabled={unlockLoading}>
                                                        {unlockLoading ? "解锁中…" : "解锁配置"}
                                                </button>
                                        </form>
                                ) : null}
                                <MessageBar message={message} />
                        </header>
		{activeTab === "settings" ? (
			<div className="settings-container">
				<ConfigForm
						draft={configDraft}
						onFieldChange={handleConfigFieldChange}
						onSubmit={handleConfigSubmit}
						onReset={handleConfigReset}
						saving={configSaving}
						locked={isConfigLocked}
                                                activeSection={configTab}
                                                onSectionChange={handleConfigSectionChange}
                                                onImport={handleConfigImportClick}
                                                onExport={handleConfigExport}
                                                importing={configImporting}
                                                exporting={configExporting}
				/>
                                <input
                                        ref={configImportInputRef}
                                        type="file"
                                        accept="application/json"
                                        style={{ display: "none" }}
                                        onChange={handleConfigImportFile}
                                />
				<section className="password-section">
					<h2>配置密码</h2>
                                                {configState.hasPassword ? (
                                                        <div className="password-form">
                                                                <input
                                                                        type="password"
                                                                        value={passwordInputs.oldPassword}
                                                                        placeholder="当前密码"
                                                                        onChange={(event) => handlePasswordInputChange("oldPassword", event.target.value)}
                                                                        disabled={passwordSaving}
                                                                />
                                                                <input
                                                                        type="password"
                                                                        value={passwordInputs.newPassword}
                                                                        placeholder="新密码（至少 8 位）"
                                                                        onChange={(event) => handlePasswordInputChange("newPassword", event.target.value)}
                                                                        disabled={passwordSaving}
                                                                />
                                                                <button type="button" onClick={handleChangePassword} disabled={passwordSaving}>
                                                                        {passwordSaving ? "处理中…" : "修改密码"}
                                                                </button>
                                                        </div>
                                                ) : (
                                                        <div className="password-form">
                                                                <input
                                                                        type="password"
                                                                        value={passwordInputs.password}
                                                                        placeholder="设置新密码（至少 8 位）"
                                                                        onChange={(event) => handlePasswordInputChange("password", event.target.value)}
                                                                        disabled={passwordSaving}
                                                                />
                                                                <button type="button" onClick={handleSetPassword} disabled={passwordSaving}>
                                                                        {passwordSaving ? "处理中…" : "设置密码"}
                                                                </button>
                                                                <div className="field-hint">配置将使用该密码加密存储，请妥善保管。</div>
                                                        </div>
                                                )}
                                        </section>
                                </div>
                        ) : (
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
                        )}
                </React.Fragment>
        );
}

export default App;
