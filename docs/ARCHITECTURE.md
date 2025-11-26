# 项目架构概览

本文档梳理 `openai-backup` 的整体结构，帮助快速理解各模块职责与主要数据流。

## 顶层目录

```
openai-backup/
├─ anytype.go         # Anytype API 客户端与同步逻辑
├─ client.go          # ChatGPT 会话列表/详情/删除接口封装
├─ export.go          # 会话内容归一化、Markdown 渲染等导出工具
├─ logger.go          # 日志初始化与辅助函数
├─ main.go            # 应用入口，加载配置后启动 Web
├─ notion.go          # Notion API 客户端与同步逻辑
├─ server.go          # Web 服务端路由、配置管理、缓存、持久化调度
├─ store.go           # SQLite 持久化与加解密
├─ types.go           # ChatGPT/导出结构体定义
├─ web/               # Vite + React 前端工程
└─ scripts/           # 编译、打包、运行脚本
```

## 运行模式

项目以 Web 服务模式运行：  
- `main.go` 启动 Web Server，暴露 REST API、前端静态资源并初始化配置存储。  
- `server.go` 协调配置加载/保存、缓存 ChatGPT 列表/详情、触发导入任务。  
- 前端 `web/` 目录构建出的静态页面由 `server.go` 的 `serveIndex` 提供。  
- `cliConfig` 作为 Web 配置载体，同时用于环境变量/启动参数的默认值收敛。

## 后端模块拆解

- **`main.go`**：解析参数 → 初始化日志 → 构建 HTTP 客户端 → 跳转导出或 Web 模式。  
- **`client.go`**：  
  - `fetchConversationPage`/`fetchConversationDetail` 调用 ChatGPT 官方接口，统一注入鉴权头（`applyCommonHeaders`）。  
  - `deleteConversation` 封装删除接口。  
- **`server.go`**：  
  - 维护配置、列表缓存与详情缓存（`conversationPageCacheEntry`、`detailCacheEntry`）。  
  - 调度 `store.go` 完成配置加载/持久化，并暴露 `/api/config`、`/api/config/export`、`/api/config/import`、`/api/conversations`、`/api/import`、`/api/conversations/delete` 等端点。  
  - 将前端 build 产物嵌入 `embed.FS`，无外部依赖即可运行。  
- **`store.go`**：封装 SQLite 持久化逻辑，提供配置的读写接口。
- **`export.go`**：  
  - `buildExportConversation` 抽取 ChatGPT 消息树，过滤空节点，按时间排序。  
  - `renderConversationMarkdown`/`renderMessageContent` 负责 Markdown 化消息文本。  
- **`anytype.go` / `notion.go`**：将归一化后的对话写入目标系统。  
- **`logger.go`**：统一的日志输出。  
- **`types.go`**：保存 ChatGPT 原始结构、导出结构等类型定义。

## 前端结构

- 使用 Vite + React + 原生 CSS，入口文件 `web/src/main.jsx`，核心页面在 `web/src/App.jsx`。  
- 主要功能：
  - 读取/保存服务端配置。  
  - 拉取会话列表、预览详情。  
  - 选择目标（Anytype / Notion）并触发导入。  
  - 删除会话、刷新列表。  
  - 提供导入/导出配置、前端直连测试等工具。  
  - 新增的“前端直连测试”直接向 ChatGPT `conversations` 接口发请求，验证鉴权配置。  
- 构建后产物位于 `web/dist`，由 Go 服务通过 `embed` 提供。

## 配置与状态

- `cliConfig` 同时作为启动参数载体与 Web 配置结构。  
- Web 模式通过 `store.go` 把配置写入 SQLite（明文存储，方便排查与备份）。  
- 会话缓存 TTL：列表 30 秒、详情 5 分钟，减少频繁访问官方接口；SQLite 文件可随时备份/恢复以同步配置状态。

## 数据流概览

```
ChatGPT API ── client.go ──► 会话元数据
    │                          │
    │                          └─► export.go 归一化消息
    │                                      │
    └─ server.go (缓存 + REST) ◄───────────┤
                                           ├─► anytype.go / notion.go → 目标平台
                                           └─► web/src/App.jsx → 浏览、筛选、导入
```

上述文档配合 `docs/BUILD_AND_RUN.md` 中的命令，可快速完成编译、打包与运行。  
