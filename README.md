# openai-backup

`openai-backup` 用于从 ChatGPT 导出对话，并同步到 Anytype / Notion，提供内置 Web 界面完成配置、预览与导入。

- **架构说明**：详见 `docs/ARCHITECTURE.md`，涵盖后端模块划分、数据流、前端结构。  
- **构建与运行**：详见 `docs/BUILD_AND_RUN.md`，提供脚本与常用命令。

## 快速开始

```bash
# 1. 编译后端
./scripts/build-backend.sh

# 2. 构建前端
./scripts/build-frontend.sh

# 3. 设置 Token 后启动 Web
export CHATGPT_BEARER_TOKEN="sk-..."
./scripts/run-serve.sh --listen 127.0.0.1:8080
```

服务会在 `config/app.db` 持久化配置（SQLite），可直接备份或迁移。  

更多参数与目标平台配置说明请参考 Web 配置页和 `constants.go`。  

## 配置存储

- Web 模式下的配置保存在 `config/app.db`（SQLite），可直接备份或迁移。  
- 也可通过环境变量（如 `CHATGPT_BEARER_TOKEN`、`ANYTYPE_TOKEN`、`NOTION_TOKEN` 等）或启动参数（如 `--listen`、`--base-url`）提供默认值，保存后写入 SQLite。  
