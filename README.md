# openai-backup

`openai-backup` 用于从 ChatGPT 导出对话，并同步到 Anytype / Notion，支持 CLI 与内置 Web 界面两种模式。

- **架构说明**：详见 `docs/ARCHITECTURE.md`，涵盖后端模块划分、数据流、前端结构。  
- **构建与运行**：详见 `docs/BUILD_AND_RUN.md`，提供脚本与常用命令。

## 快速开始

```bash
# 1. 编译后端
./scripts/build-backend.sh

# 2. 构建前端
./scripts/build-frontend.sh

# 3. 设置 Token 后启动 Web（可选配置密码自动预设）
export CHATGPT_BEARER_TOKEN="sk-..."
# 可选：若希望自动解锁配置，可提前设置 OPENAIBACKUP_CONFIG_SECRET
./scripts/run-serve.sh --listen 127.0.0.1:8080
```

服务会在 `config/app.db` 持久化配置（AES-GCM 加密），首次运行可在 Web 界面设置/修改密码，也可通过 `OPENAIBACKUP_CONFIG_SECRET` 自动预设。数据库文件可直接备份迁移。  

更多 CLI 参数、目标平台配置及加密密钥说明请参考 `go run main.go --help` 与 `constants.go`。  

## 配置文件

- CLI 模式会优先加载配置文件（JSON），默认路径遵循系统约定：`macOS -> ~/Library/Application Support/openai-backup/config.json`、`Windows -> %APPDATA%\openai-backup\config.json`、`Linux -> ~/.config/openai-backup/config.json`。若目录不存在将被忽略。  
- 可通过 `--config` 指定文件或目录，例如 `--config ~/my-config` 将自动查找 `~/my-config/config.json`。命令行参数始终优先生效，其次是配置文件，再回退到环境变量与内置默认值。  
- 示例：

```json
{
  "token": "sk-xxx",
  "export_target": "notion",
  "notion_parent_id": "xxxxxxxxxxxxxxxxxxxx",
  "page_size": 50,
  "include_archived": true
}
```
