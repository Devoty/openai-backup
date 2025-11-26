import { initialConfig } from "../config/constants";

export function normalizeTarget(value) {
	const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
	return lower === "notion" ? "notion" : "anytype";
}

export function sanitizeOrder(value) {
	const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
	return lower === "created" ? "created" : "updated";
}

export function sanitizeParentType(value) {
	const lower = typeof value === "string" ? value.trim().toLowerCase() : "";
	if (lower === "page" || lower === "database") {
		return lower;
	}
	return "";
}

export function toNumber(value) {
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

export function clampPageSizeValue(value) {
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

export function normalizeConfigResponse(data) {
	const normalized = { ...initialConfig };
	if (!data || typeof data !== "object") {
		return normalized;
	}
	const assignString = (key) => {
		if (typeof data[key] === "string") {
			normalized[key] = data[key];
		}
	};

	const keysToAssign = [
		"listen",
		"timezone",
		"base_url",
		"token",
		"device_id",
		"user_agent",
		"accept_language",
		"referer",
		"cookie",
		"origin",
		"oai_language",
		"sec_ch_ua",
		"sec_ch_ua_mobile",
		"sec_ch_ua_platform",
		"sec_fetch_dest",
		"sec_fetch_mode",
		"sec_fetch_site",
		"chatgpt_account_id",
		"oai_client_version",
		"priority",
		"log_path",
		"anytype_base_url",
		"anytype_version",
		"anytype_space_id",
		"anytype_type_key",
		"anytype_token",
		"notion_base_url",
		"notion_version",
		"notion_token",
		"notion_parent_id",
		"notion_title_property"
	];
	keysToAssign.forEach(assignString);

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

export function createConfigDraft(config) {
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

export function prepareConfigPayload(draft) {
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
