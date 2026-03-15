# Audit Logging System

安全事件审计日志系统，用于记录和追溯关键安全事件。

## 功能特性

- ✅ **结构化日志**: JSON 格式，易于解析和分析
- ✅ **异步写入**: channel + 后台 goroutine，避免阻塞主流程
- ✅ **日志轮转**: 按文件大小自动轮转，防止单个文件过大
- ✅ **自动清理**: 定期删除旧日志（默认 30 天）
- ✅ **敏感信息脱敏**: API Key、密码、Token 等自动脱敏
- ✅ **RequestID 追踪**: 支持分布式追踪
- ✅ **上下文集成**: 通过 context 传递审计信息
- ✅ **便捷函数**: 预定义的常见事件日志函数

## 事件类型

### 认证事件
- `auth.success` - 认证成功
- `auth.failure` - 认证失败

### 限流事件
- `ratelimit.violated` - 限流违规

### 权限事件
- `permission.changed` - 权限变更
- `permission.granted` - 权限授予
- `permission.revoked` - 权限撤销

### 配置事件
- `config.modified` - 配置修改
- `config.created` - 配置创建
- `config.deleted` - 配置删除

### 系统事件
- `system.start` - 系统启动
- `system.stop` - 系统停止
- `system.restart` - 系统重启

## 快速开始

### 1. 初始化审计日志器

```go
package main

import (
    "github.com/wangminggit/distributed-bloom-filter/internal/audit"
)

func main() {
    // 使用默认配置初始化
    err := audit.Init()
    if err != nil {
        log.Fatalf("Failed to initialize audit logger: %v", err)
    }
    
    // 或使用自定义配置
    config := audit.LoggerConfig{
        LogDir:        "logs/audit",
        MaxFileSize:   10 * 1024 * 1024, // 10MB
        MaxAge:        30 * 24 * time.Hour, // 30 天
        BufferSize:    1000,
        FlushInterval: 5 * time.Second,
        EnableConsole: false,
    }
    
    logger, err := audit.NewLogger(config)
    if err != nil {
        log.Fatalf("Failed to create logger: %v", err)
    }
    defer logger.Close()
}
```

### 2. 记录审计事件

#### 使用便捷函数

```go
// 认证成功
audit.LogAuthSuccess("192.168.1.100", "user123", "/proto.DBFService/Check")

// 认证失败
audit.LogAuthFailure("192.168.1.100", "user123", "/proto.DBFService/Check", "Invalid API key")

// 限流违规
audit.LogRateLimitViolation("192.168.1.100", "user123", "/proto.DBFService/Check")

// 权限变更
audit.LogPermissionChange("192.168.1.100", "admin", "grant", "user456", "read")

// 配置修改
audit.LogConfigChange("192.168.1.100", "admin", "rate_limit", 100, 200)
```

#### 使用构建器模式

```go
event := audit.NewAuditEvent(audit.EventAuthSuccess, audit.SeverityInfo).
    WithClientIP("192.168.1.100").
    WithUserID("user123").
    WithMethod("/proto.DBFService/Check").
    WithResult("success").
    WithReason("Authentication successful").
    WithRequestID("req-abc123").
    WithMetadata("api_key", audit.SanitizeAPIKey(apiKey))

logger.Log(event)
```

#### 同步写入（关键事件）

```go
err := logger.LogSync(event)
if err != nil {
    log.Printf("Failed to write audit event: %v", err)
}
```

### 3. gRPC 拦截器集成

```go
package grpc

import (
    "github.com/wangminggit/distributed-bloom-filter/internal/audit"
)

func NewGRPCServer(config ServerConfig) (*GRPCServer, error) {
    // 初始化审计日志器
    auditConfig := audit.LoggerConfig{
        LogDir:      config.AuditLogDir,
        MaxFileSize: config.AuditMaxFileSize,
        MaxAge:      time.Duration(config.AuditMaxAge) * 24 * time.Hour,
    }
    
    auditLogger, err := audit.NewLogger(auditConfig)
    if err != nil {
        return nil, err
    }
    
    // 创建审计拦截器
    auditInterceptor := audit.NewAuditInterceptor(auditLogger)
    
    // 创建认证拦截器
    authInterceptor := NewAuthInterceptor(config.APIKeyStore)
    auditAuthInterceptor := audit.NewAuditAuthInterceptor(authInterceptor, auditLogger)
    
    // 创建限流拦截器
    rateInterceptor := NewRateLimitInterceptor(config.RateLimitPerSecond, config.RateLimitBurstSize)
    auditRateInterceptor := audit.NewAuditRateLimitInterceptor(rateInterceptor, auditLogger)
    
    // 链式拦截器（顺序很重要！）
    opts := []grpc.ServerOption{
        grpc.ChainUnaryInterceptor(
            auditInterceptor.UnaryInterceptor(),     // 1. 审计（请求追踪）
            auditAuthInterceptor.UnaryInterceptor(), // 2. 认证 + 审计
            auditRateInterceptor.UnaryInterceptor(), // 3. 限流 + 审计
        ),
    }
    
    server := grpc.NewServer(opts...)
    // ...
}
```

### 4. 上下文传递

```go
import "github.com/wangminggit/distributed-bloom-filter/internal/audit"

// 在请求入口处设置审计信息
ctx := audit.ContextWithAuditInfo(
    context.Background(),
    "req-abc123",    // RequestID
    "192.168.1.100", // ClientIP
    "user123",       // UserID
)

// 在后续处理中提取审计信息
requestID, clientIP, userID := audit.GetAuditInfoFromContext(ctx)
```

## 日志格式

### JSON 结构

```json
{
  "timestamp": 1710403200000,
  "time": "2026-03-14T13:00:00+08:00",
  "event_type": "auth.success",
  "severity": "INFO",
  "client_ip": "192.168.1.100",
  "user_id": "user123",
  "method": "/proto.DBFService/Check",
  "result": "success",
  "reason": "Authentication successful",
  "metadata": {
    "duration_ms": 15,
    "api_key": "sk-1***...***cdef"
  },
  "request_id": "req-abc123"
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| timestamp | int64 | Unix 时间戳（毫秒） |
| time | string | ISO 8601 格式时间 |
| event_type | string | 事件类型 |
| severity | string | 严重性级别 (INFO/WARNING/ERROR/CRITICAL) |
| client_ip | string | 客户端 IP 地址 |
| user_id | string | 用户 ID |
| method | string | gRPC 方法或 API 端点 |
| result | string | 结果 (success/failure/violation 等) |
| reason | string | 事件原因/描述 |
| metadata | object | 额外的元数据 |
| request_id | string | 请求追踪 ID |

## 敏感信息脱敏

### API Key 脱敏

```go
apiKey := "sk-1234567890abcdef"
sanitized := audit.SanitizeAPIKey(apiKey)
// 输出：sk-1***********cdef
```

### 通用值脱敏

```go
password := "mysecretpassword"
sanitized := audit.SanitizeValue(password)
// 输出：**************word
```

### 配置值脱敏

```go
configValue := "super-secret-token"
sanitized := audit.SanitizeConfigValue(configValue)
// 自动检测敏感关键词并脱敏
```

## 日志轮转

### 按大小轮转

当日志文件达到 `MaxFileSize` 时自动轮转：

```go
config := audit.LoggerConfig{
    MaxFileSize: 10 * 1024 * 1024, // 10MB
}
```

### 按时间轮转

日志文件名包含时间戳，便于按时间归档：

```
audit_20260314_100000.log
audit_20260314_110000.log
audit_20260314_120000.log
```

### 自动清理

定期删除超过 `MaxAge` 的旧日志：

```go
config := audit.LoggerConfig{
    MaxAge: 30 * 24 * time.Hour, // 30 天
}
```

## 最佳实践

### 1. 尽早初始化

在应用启动时立即初始化审计日志器：

```go
func main() {
    if err := audit.Init(); err != nil {
        log.Fatalf("Failed to initialize audit logger: %v", err)
    }
    defer audit.GetLogger().Close()
    
    // ... 应用逻辑
}
```

### 2. 优雅关闭

确保应用退出前刷新所有缓冲的日志：

```go
logger := audit.GetLogger()
if logger != nil {
    if err := logger.Close(); err != nil {
        log.Printf("Error closing audit logger: %v", err)
    }
}
```

### 3. 使用 RequestID 追踪

为每个请求生成唯一的 RequestID 并传递到整个调用链：

```go
requestID := uuid.New().String()
ctx := audit.ContextWithAuditInfo(context.Background(), requestID, clientIP, userID)
```

### 4. 敏感信息脱敏

记录任何可能包含敏感信息的字段前都要脱敏：

```go
// ✅ 正确
event.WithMetadata("api_key", audit.SanitizeAPIKey(apiKey))

// ❌ 错误 - 泄露敏感信息
event.WithMetadata("api_key", apiKey)
```

### 5. 关键事件同步写入

对于非常重要的安全事件，使用同步写入确保不丢失：

```go
if isCriticalEvent(event) {
    logger.LogSync(event)
} else {
    logger.Log(event)
}
```

## 性能考虑

- **异步写入**: 默认使用 channel 缓冲，不会阻塞主流程
- **批量刷新**: 定期刷新缓冲（默认 5 秒），减少磁盘 I/O
- **缓冲大小**: 根据负载调整 `BufferSize`（默认 1000）
- **并发安全**: 所有方法都是并发安全的

## 监控和告警

建议监控以下指标：

1. **日志文件大小**: 检测异常增长
2. **缓冲区使用率**: 检测写入压力
3. **特定事件频率**: 如认证失败、限流违规等
4. **磁盘空间**: 确保有足够空间存储日志

## 故障排查

### 日志未写入

1. 检查日志目录权限
2. 检查磁盘空间
3. 查看控制台错误日志
4. 确认 logger 已正确初始化

### 缓冲区满

如果看到 "buffer full, dropping event" 警告：

1. 增加 `BufferSize`
2. 减少 `FlushInterval`
3. 检查是否有大量事件涌入

### 日志轮转失败

1. 检查文件权限
2. 确保磁盘空间充足
3. 查看系统日志

## 安全注意事项

1. **保护日志文件**: 审计日志包含敏感信息，应限制访问权限
2. **加密存储**: 考虑对日志文件进行加密
3. **远程备份**: 定期将日志备份到安全位置
4. **访问控制**: 只有授权人员可以查看审计日志
5. **完整性保护**: 考虑使用数字签名防止篡改

## 测试

运行审计日志测试：

```bash
go test ./internal/audit/... -v
```

## 相关文件

- `events.go` - 事件类型定义
- `logger.go` - 审计日志核心实现
- `logger_test.go` - 测试用例
- `README.md` - 本文档

## 参考资料

- [NIST SP 800-92 Guide to Computer Security Log Management](https://csrc.nist.gov/publications/detail/sp/800-92/final)
- [CIS Controls - Log Management](https://www.cisecurity.org/control/log-management)
