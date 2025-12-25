# kiro2api

**AI API 代理** - 桥接 Anthropic/OpenAI 与 AWS CodeWhisperer

[![Go 1.24+](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org/)

## 快速开始

```bash
# 编译
cd backend
go build -o kiro2api ./cmd/kiro2api

# 配置
export KIRO_CLIENT_TOKEN="your-api-key"
export KIRO_AUTH_TOKEN='[{"auth":"Social","refreshToken":"your-token"}]'

# 运行（⚠️ 必须从 backend 目录，不能从项目根目录运行）
./kiro2api
```

> ⚠️ **重要**: 必须从 `backend/` 目录运行程序。数据库路径为相对路径 `./data/kiro2api.db`，从根目录运行会创建空数据库导致 Token 加载失败。

**项目结构**：
```
kiro2api/
├── backend/          # 后端代码和数据
│   ├── data/         # 数据库文件
│   └── kiro2api      # 编译后的二进制
├── frontend/         # 前端代码
└── README.md
```

**Claude Code 集成**:
```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/v1"
export ANTHROPIC_API_KEY="your-api-key"
```

## 核心特性

- **双 API 兼容**: Anthropic + OpenAI 格式
- **多账号池**: 顺序选择、故障转移、冷却机制
- **双认证**: Social + IdC
- **流式响应**: SSE 零延迟
- **自动重试**: 429/5xx 自动切换 Token

## API 端点

### 基础端点

| 端点 | 说明 |
|------|------|
| `GET /v1/models` | 模型列表 |
| `POST /v1/messages` | Anthropic API（使用 default 分组） |
| `POST /v1/chat/completions` | OpenAI API（使用 default 分组） |
| `GET /api/tokens` | Token 池状态 |

### 分组端点

**⚠️ 重要**：默认情况下，所有请求使用 `default` 分组。要使用其他分组（如 `pro`），需要在路径中指定分组名称：

| 端点 | 说明 |
|------|------|
| `POST /:group/v1/messages` | 使用指定分组的 Anthropic API |
| `POST /:group/v1/chat/completions` | 使用指定分组的 OpenAI API |

**示例**：
```bash
# 使用 default 分组（默认）
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer 123456" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[...]}'

# 使用 pro 分组
curl -X POST http://localhost:8080/pro/v1/messages \
  -H "Authorization: Bearer 123456" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[...]}'
```

**Claude Code 配置**：
```bash
# 使用 default 分组
export ANTHROPIC_BASE_URL="http://localhost:8080/v1"

# 使用 pro 分组
export ANTHROPIC_BASE_URL="http://localhost:8080/pro/v1"
```

## 支持模型

| 模型 | 内部 ID |
|------|---------|
| `claude-opus-4-5-20251101` | `claude-opus-4.5` |
| `claude-sonnet-4-5-20250929` | `CLAUDE_SONNET_4_5_20250929_V1_0` |
| `claude-sonnet-4-20250514` | `CLAUDE_SONNET_4_20250514_V1_0` |
| `claude-3-7-sonnet-20250219` | `CLAUDE_3_7_SONNET_20250219_V1_0` |
| `claude-haiku-4-5` | `auto` |

<details>
<summary><b>多账号配置</b></summary>

```bash
# 混合认证
export KIRO_AUTH_TOKEN='[
  {"auth":"Social","refreshToken":"token1"},
  {"auth":"Social","refreshToken":"token2"},
  {"auth":"IdC","refreshToken":"idc-token","clientId":"xxx","clientSecret":"xxx"}
]'
```

Token 位置: `~/.aws/sso/cache/kiro-auth-token.json`

</details>

<details>
<summary><b>Docker 部署</b></summary>

```bash
# docker-compose
docker-compose up -d

# 或直接运行
docker run -d -p 8080:8080 \
  -e KIRO_AUTH_TOKEN='[{"auth":"Social","refreshToken":"xxx"}]' \
  -e KIRO_CLIENT_TOKEN="123456" \
  ghcr.io/caidaoli/kiro2api:latest
```

</details>

<details>
<summary><b>环境变量</b></summary>

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `KIRO_CLIENT_TOKEN` | API 认证密钥 | - |
| `KIRO_AUTH_TOKEN` | AWS Token 配置 | - |
| `KIRO_DB_PATH` | 数据库文件路径（可选） | `./data/kiro2api.db` |
| `PORT` | 服务端口 | 8080 |
| `LOG_LEVEL` | 日志级别 | info |
| `GIN_MODE` | 运行模式 | release |
| `MAX_TOOL_DESCRIPTION_LENGTH` | 工具描述限制 | 10000 |

> **注意**: 数据库路径相对于 `backend/` 目录。必须从 `backend/` 目录运行程序。

</details>

<details>
<summary><b>请求示例</b></summary>

```bash
# Anthropic 格式
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer 123456" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":100,"messages":[{"role":"user","content":"你好"}]}'

# OpenAI 格式
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer 123456" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"你好"}]}'
```

</details>

<details>
<summary><b>架构图</b></summary>

```
Client (Anthropic/OpenAI) → API Gateway → 格式转换 → Token 管理 → CodeWhisperer
                                ↓
                          认证中间件
                                ↓
                          流式处理器
```

</details>

## 开发

```bash
cd backend
go test ./...              # 测试
go vet ./...               # 检查
LOG_LEVEL=debug ./kiro2api # 调试模式
```

详见 [CLAUDE.md](./CLAUDE.md)

## License

MIT
