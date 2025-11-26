# 编译 / 打包 / 运行指南

本文档给出常用脚本与命令，帮助快速完成后端与前端的构建与运行。

## 环境要求

- Go 1.24+（以支持当前 `go.mod` 及 modernc SQLite 驱动）  
- Node.js 18+、npm 9+（用于前端构建）  
- 可选：`make` 或 `bash`，用于执行脚本

## 快速脚本

项目内置 `scripts/` 目录，封装了常见操作：

| 操作 | 脚本 | 说明 |
| --- | --- | --- |
| 编译 Go 后端 | `scripts/build-backend.sh` | 在 `bin/openai-backup` 生成可执行文件 |
| 构建前端静态资源 | `scripts/build-frontend.sh` | 自动执行 `npm install` + `npm run build`，结果位于 `web/dist` |
| 启动 Web 服务 | `scripts/run-serve.sh [--listen 0.0.0.0:8080 ...]` | 启动内置 Web，可在界面中维护配置并触发导入 |

> 以上脚本默认在仓库根目录执行，必要时可结合环境变量（例如 `CHATGPT_BEARER_TOKEN`）使用。

## 手动命令

若需要自定义流程，可直接使用以下命令：

### 编译后端

```bash
go build -o bin/openai-backup ./...
```

### 构建前端

```bash
cd web
npm install
npm run build
```

### Web 模式

```bash
export CHATGPT_BEARER_TOKEN="sk-..."
./bin/openai-backup --listen 127.0.0.1:8080
```

首次访问 http://127.0.0.1:8080/ 将显示前端界面，可在“设置”页补充请求头、目标平台信息，保存后立即生效。

### 前端开发模式

```bash
cd web
npm install
npm run dev
```

Vite 默认监听 `http://localhost:5173`，如需调试与后端同源服务，可结合浏览器代理或 `vite.config.js` 中的代理配置。

## 运行前检查

1. **Token 环境变量**：确保 `CHATGPT_BEARER_TOKEN` 可用；也可在设置页填写后持久化。  
2. **ChatGPT 请求头**：若账号需要额外头部（`oai-device-id`、`User-Agent` 等），请在 Web 设置中补齐。  
3. **目标平台凭证**：导出到 Anytype / Notion 前，提前准备 API Key 与空间/父级 ID。  
4. **前端构建产物与配置备份**：`run-serve.sh` 仅依赖 Go `embed` 中的 `web/dist`，若更新前端记得重新执行 build。配置数据库位于 `config/app.db`，可直接备份或挂载。

更多细节和模块关系请参考 `docs/ARCHITECTURE.md`。  
