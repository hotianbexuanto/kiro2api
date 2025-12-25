# 伪装配置文档

## 概述

kiro2api 通过伪装成 Kiro IDE 客户端与 AWS CodeWhisperer API 通信。所有伪装参数集中在 `backend/internal/config/config.go` 中管理。

## 动态指纹

**文件**: `backend/internal/config/fingerprint.go`

每个 Token 使用独立指纹，避免多账号共用同一指纹被检测：

```go
// 获取指纹管理器
fpManager := config.GetFingerprintManager()

// 为 token 生成/获取指纹（相同 tokenID 返回相同指纹）
tokenID := refreshToken[:16]  // 使用 refreshToken 前16字符作为标识
fingerprint := fpManager.GenerateFingerprint(tokenID)
```

**特性**：
- 相同 tokenID 始终返回相同指纹（确定性）
- 不同 tokenID 生成不同指纹
- 指纹格式：64字符十六进制（SHA256）

## 统一常量

**文件**: `backend/internal/config/config.go`

```go
const (
    KiroSDKVersion  = "1.0.27"      // AWS SDK 版本
    KiroIDEVersion  = "0.8.0"       // Kiro IDE 版本
    KiroFingerprint = "66c23a8c..."  // 默认指纹 (动态生成优先)
    KiroOS          = "win32#10.0.19044"  // 操作系统
    KiroNodeVersion = "22.17.0"     // Node.js 版本
)
```

### 参数说明

| 参数 | 当前值 | 说明 | 更新频率 |
|------|--------|------|----------|
| `KiroSDKVersion` | 1.0.27 | AWS SDK JS 版本 | 随 Kiro 更新 |
| `KiroIDEVersion` | 0.8.0 | Kiro IDE 版本号 | 每次 Kiro 发版 |
| `KiroOS` | win32#10.0.19044 | 操作系统标识 | 少变 |
| `KiroNodeVersion` | 22.17.0 | Node.js 运行时版本 | 随 Kiro 更新 |

### User-Agent 格式 (AWS SDK JS v3)

```
aws-sdk-js/{SDK} ua/2.1 os/{OS} lang/js md/nodejs#{Node} api/{service}#{SDK} m/E {custom}
```

**组件解析**：
- `aws-sdk-js/{SDK}` - SDK 标识和版本
- `ua/2.1` - User-Agent 规范版本（固定）
- `os/{platform}#{release}` - 操作系统和版本
- `lang/js` - 语言标识（固定）
- `md/nodejs#{version}` - Node.js 运行时版本
- `api/{service}#{version}` - API 服务标识
- `m/E` - 执行环境标记（固定）
- `{custom}` - 自定义标识（KiroIDE-版本-指纹）

## 请求头格式

### CodeWhisperer API (`backend/internal/service/request_executor.go`)

```
User-Agent: aws-sdk-js/{SDK} ua/2.1 os/{OS} lang/js md/nodejs#{Node} api/codewhispererstreaming#{SDK} m/E KiroIDE-{IDE}-{FP}

x-amz-user-agent: aws-sdk-js/{SDK} KiroIDE-{IDE}-{FP}
```

**示例**:
```
User-Agent: aws-sdk-js/1.0.27 ua/2.1 os/win32#10.0.19044 lang/js md/nodejs#22.17.0 api/codewhispererstreaming#1.0.27 m/E KiroIDE-0.8.0-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1

x-amz-user-agent: aws-sdk-js/1.0.27 KiroIDE-0.8.0-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1
```

### 使用限制检查 (`backend/internal/auth/usage_checker.go`)

```
User-Agent: aws-sdk-js/{SDK} ua/2.1 os/{OS} lang/js md/nodejs#{Node} api/codewhispererruntime#{SDK} m/E KiroIDE-{IDE}-{FP}

x-amz-user-agent: aws-sdk-js/{SDK} KiroIDE-{IDE}-{FP}
```

**注意**: API 名称为 `codewhispererruntime` (非 streaming)

### IdC Token 刷新 (`backend/internal/auth/refresh.go`)

```
User-Agent: node
x-amz-user-agent: aws-sdk-js/3.738.0 ua/2.1 os/other lang/js md/browser#unknown_unknown api/sso-oidc#3.738.0 m/E KiroIDE
```

**特殊设计**: IdC OIDC 端点期望浏览器风格请求，使用独立配置。

## 其他伪装头

| 头名称 | 值 | 用途 |
|--------|-----|------|
| `x-amzn-kiro-agent-mode` | `vibe` | Kiro 代理模式 |
| `x-amzn-codewhisperer-optout` | `true` | 数据收集退出 |
| `amz-sdk-invocation-id` | UUID | 请求追踪 ID |
| `amz-sdk-request` | `attempt=1; max=3` | 重试策略 |

## 文件依赖关系

```
backend/internal/config/config.go       ← 版本常量
backend/internal/config/fingerprint.go  ← 动态指纹生成
    │
    ├── backend/internal/service/request_executor.go (CodeWhisperer API - 动态指纹)
    ├── backend/internal/auth/usage_checker.go       (使用限制检查 - 动态指纹)
    └── backend/internal/auth/refresh.go             (IdC 刷新 - 独立配置)
```

## 版本更新流程

当 Kiro IDE 发布新版本时：

1. **获取新版本信息**
   - 查看 Kiro 更新日志: https://kiro.dev/changelog/
   - 或从 Kiro IDE 客户端抓包获取最新 User-Agent

2. **更新常量**
   ```go
   // backend/internal/config/config.go
   KiroSDKVersion  = "新版本"
   KiroIDEVersion  = "新版本"
   KiroOS          = "新系统"  // 如有变化
   KiroNodeVersion = "新版本"  // 如有变化
   ```

   **注意**: 指纹现在动态生成，无需手动更新

3. **验证**
   ```bash
   cd backend
   go test ./...
   go build -o kiro2api ./cmd/kiro2api
   ```

4. **测试请求**
   ```bash
   curl -X POST http://localhost:8080/v1/messages ...
   ```

## 检查清单

- [ ] `backend/internal/config/config.go` - 常量已更新
- [ ] `backend/internal/service/request_executor.go` - 使用 `config.Kiro*` 常量
- [ ] `backend/internal/auth/usage_checker.go` - 使用 `config.Kiro*` 常量
- [ ] `backend/internal/auth/refresh.go` - IdC 独立配置（通常不需要改）
- [ ] 编译通过
- [ ] 测试通过
- [ ] 实际请求验证

## 故障排查

### 账号被封

可能原因：
1. **伪装不一致** - 检查所有请求头是否统一
2. **请求 ID 暴露** - 确保使用纯 UUID，不含项目标识
3. **IP 异常** - 服务器 IP vs 普通用户 IP
4. **请求频率** - 检查限流配置

### 验证伪装

```bash
# 查看实际发送的请求头
cd backend && GIN_MODE=debug LOG_LEVEL=debug ./kiro2api
```

## 历史版本

| 日期 | SDK | IDE | Node | 备注 |
|------|-----|-----|------|------|
| 2025-12-22 | 1.0.27 | 0.8.0 | 22.17.0 | 当前配置 |
| 2025-12 | 1.0.27 | 0.7.45 | 22.21.1 | 旧版本 |

## 自动版本检测

运行脚本自动检测本地 Kiro IDE 版本：

```bash
./scripts/check_kiro_version.sh
```

输出示例：
```
=== Kiro IDE 版本信息 ===
IDE 版本: 0.8.0
Node 版本: 22.17.0
SDK 版本: 1.0.27

=== 对比 ===
✓ IDE 版本一致
✓ Node 版本一致
✓ SDK 版本一致
```

## 风险参数

以下参数可能影响检测：

| 参数 | 风险等级 | 说明 |
|------|----------|------|
| 指纹 | 高 | 已实现动态生成 ✓ |
| IDE 版本 | 中 | 自动检测 ✓ |
| SDK 版本 | 中 | 自动检测 ✓ (来自 @aws/codewhisperer-streaming-client) |
| OS 标识 | 低 | 多样化可选 |
| Node 版本 | 低 | 自动检测 ✓ |
