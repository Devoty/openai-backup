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
| 命令行导出 | `scripts/run-export.sh [参数...]` | 复用 `bin/openai-backup`，可带任意 CLI 参数 |
| 启动 Web 服务 | `scripts/run-serve.sh [--listen 0.0.0.0:8080 ...]` | 启动内置 Web；密码可在界面设置，也可提前通过环境变量预设 |

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

### CLI 导出模式

```bash
# 确保设置 ChatGPT Bearer Token
export CHATGPT_BEARER_TOKEN="sk-..."
./bin/openai-backup \
  --target anytype \
  --timezone UTC \
  --page-size 20
```

常用参数与环境变量请参考 `main.go` / `constants.go`，例如：

- `--target [anytype|notion]`：导出目标  
- `--include-archived`：是否包含归档会话  
- `--anytype-space-id`、`--notion-parent-id` 等：目标平台配置  
- 相关 Token、头信息可通过环境变量 `CHATGPT_*` `ANYTYPE_*` `NOTION_*` 提供

### Web 模式

```bash
export CHATGPT_BEARER_TOKEN="sk-..."
# 可选：若希望启动时自动解锁，可额外提供 OPENAIBACKUP_CONFIG_SECRET
./bin/openai-backup --serve --listen 127.0.0.1:8080
```

首次访问 http://127.0.0.1:8080/ 将显示前端界面，可在“设置”页补充请求头、目标平台信息，并设置配置密码；重新启动时使用相同密码即可解锁。

### 前端开发模式

```bash
cd web
npm install
npm run dev
```

Vite 默认监听 `http://localhost:5173`，如需调试与后端同源服务，可结合浏览器代理或 `vite.config.js` 中的代理配置。

## 运行前检查

1. **Token 环境变量**：确保 `CHATGPT_BEARER_TOKEN` 可用；Web 模式下在设置界面填写后同样有效。  
2. **配置密码**：首次运行可在 Web 界面设置密码并自动加密 `config/app.db`；若希望自动解锁，可在启动前提供 `OPENAIBACKUP_CONFIG_SECRET` 或 `--config-secret`。  
3. **ChatGPT 请求头**：若账号需要额外头部（`oai-device-id`、`User-Agent` 等），请在 CLI 参数或 Web 设置中补齐。  
4. **目标平台凭证**：导出到 Anytype / Notion 前，提前准备 API Key 与空间/父级 ID。  
5. **前端构建产物与配置备份**：`run-serve.sh` 仅依赖 Go `embed` 中的 `web/dist`，若更新前端记得重新执行 build。配置数据库位于 `config/app.db`，可直接备份或挂载。

更多细节和模块关系请参考 `docs/ARCHITECTURE.md`。  
