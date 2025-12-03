export const initialConfig = {
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

export const initialPreview = {
	id: "",
	title: "",
	createTime: "",
	updateTime: "",
	messages: [],
	loading: false
};

export const configSections = [
	{
		key: "core",
		title: "基础配置",
		description: "配置接口地址、监听端口、分页与最大导出数量。",
		fields: [
			{ key: "listen", label: "监听地址 / 端口", placeholder: "127.0.0.1:8080" },
			{ key: "base_url", label: "接口地址", placeholder: "https://chatgpt.com/backend-api" },
			{ key: "timezone", label: "输出时区", placeholder: "Local 或 UTC" },
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
				label: "最大导出数",
				type: "number",
				min: 0,
				description: "0 表示不限制。"
			},
			{ key: "initial_offset", label: "起始 Offset", type: "number", min: 0 },
			{ key: "include_archived", label: "包含归档对话", type: "checkbox", description: "启用后会请求已归档的对话。" },
			{
				key: "target",
				label: "默认导出目标",
				type: "select",
				options: [
					{ value: "anytype", label: "Anytype" },
					{ value: "notion", label: "Notion" }
				]
			},
			{ key: "log_path", label: "导出路径 / 日志文件", placeholder: "chatgpt_export.log", fullWidth: true }
		]
	},
	{
		key: "export",
		title: "导出设置",
		description: "配置导出路径、Notion 与 Anytype 参数。",
		fields: [
			{ key: "anytype_base_url", label: "Anytype 地址" },
			{ key: "anytype_version", label: "Anytype API 版本" },
			{ key: "anytype_space_id", label: "Anytype Space ID" },
			{ key: "anytype_type_key", label: "Anytype 类型 Key" },
			{
				key: "anytype_token",
				label: "Anytype Token",
				type: "password",
				secureToggle: true,
				fullWidth: true
			},
			{ key: "notion_base_url", label: "Notion 地址" },
			{ key: "notion_version", label: "Notion API 版本" },
			{
				key: "notion_token",
				label: "Notion Token",
				type: "password",
				secureToggle: true,
				fullWidth: true
			},
			{
				key: "notion_parent_type",
				label: "Notion 父级类型",
				type: "select",
				options: [
					{ value: "", label: "自动" },
					{ value: "page", label: "页面 (page)" },
					{ value: "database", label: "数据库 (database)" }
				]
			},
			{ key: "notion_parent_id", label: "Notion 父级 ID" },
			{ key: "notion_title_property", label: "Notion 标题属性" }
		]
	},
	{
		key: "advanced",
		title: "高级设置",
		description: "Bearer Token、UA 与安全参数，支持显示/隐藏切换。",
		fields: [
			{
				key: "token",
				label: "OpenAI Bearer Token",
				type: "password",
				secureToggle: true,
				fullWidth: true,
				description: "访问 ChatGPT 接口所需的鉴权 Token。"
			},
			{ key: "device_id", label: "Device ID" },
			{ key: "user_agent", label: "User Agent", fullWidth: true },
			{ key: "accept_language", label: "Accept-Language" },
			{ key: "referer", label: "Referer" },
			{ key: "cookie", label: "Cookie", type: "textarea", rows: 2, fullWidth: true },
			{ key: "origin", label: "Origin" },
			{ key: "oai_language", label: "OAI-Language" },
			{ key: "sec_ch_ua", label: "sec-ch-ua", fullWidth: true },
			{ key: "sec_ch_ua_mobile", label: "sec-ch-ua-mobile" },
			{ key: "sec_ch_ua_platform", label: "sec-ch-ua-platform" },
			{ key: "sec_fetch_dest", label: "sec-fetch-dest" },
			{ key: "sec_fetch_mode", label: "sec-fetch-mode" },
			{ key: "sec_fetch_site", label: "sec-fetch-site" },
			{ key: "chatgpt_account_id", label: "ChatGPT Account ID" },
			{ key: "oai_client_version", label: "OAI Client Version" },
			{ key: "priority", label: "Priority" }
		]
	}
];
