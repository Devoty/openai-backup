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
