# P0-2 TLS 加密传输实施文档

## 概述

本文档描述了分布式 Bloom 过滤器 (DBF) 项目的 P0-2 TLS 加密传输功能实施。该功能为 gRPC 通信和 Raft 节点间通信提供传输层加密。

## 实施内容

### 1. TLS 配置工具类 (`pkg/tls/config.go`)

创建了完整的 TLS 配置工具类，提供以下功能：

- **Config**: TLS 配置结构体，支持证书路径、最低 TLS 版本、加密套件等配置
- **CertReloader**: 证书热加载支持，可定期重新加载证书而无需重启服务
- **BuildTLSConfig**: 从配置构建 `tls.Config`
- **LoadTLSCertificate**: 加载 TLS 证书
- **LoadCACertPool**: 加载 CA 证书池
- **ValidateCertificate**: 验证证书有效性

**安全默认值**:
- 最低 TLS 版本：TLS 1.3
- 加密套件：仅使用安全的 AEAD 加密套件 (AES-GCM, ChaCha20-Poly1305)
- 客户端认证：RequireAndVerifyClientCert (双向认证)

### 2. gRPC 服务器 TLS 支持 (`internal/grpc/server.go`)

修改了 gRPC 服务器以支持传输层 TLS 加密：

**新增配置项**:
```go
type ServerConfig struct {
    // ... 现有字段 ...
    
    // TLS Configuration for transport layer encryption
    EnableTLS         bool
    TLSMinVersion     uint16
    TLSCertPath       string
    TLSKeyPath        string
    TLSCAPath         string
    TLSReloadInterval time.Duration
}
```

**关键实现**:
- `createTLSListener()`: 使用 `tls.NewListener()` 创建 TLS 包装的监听器
- 支持证书热加载（通过 `CertReloader`）
- 与现有的 mTLS 认证兼容（可同时启用）

### 3. Raft 通信 TLS 支持 (`internal/raft/node.go`)

为 Raft 节点间通信添加了 TLS 加密：

**新增类型**:
```go
type RaftTLSConfig struct {
    EnableTLS      bool
    CertPath       string
    KeyPath        string
    CAPath         string
    MinVersion     uint16
    ReloadInterval time.Duration
}
```

**新增方法**:
- `NewNodeWithTLS()`: 创建带 TLS 配置的 Raft 节点
- `createTLSTransport()`: 创建 TLS 包装的 Raft 传输层
- `tlsStreamLayer`: 实现 `raft.StreamLayer` 接口，提供 TLS 加密的流式传输

**实现细节**:
- 使用自定义 `tlsStreamLayer` 包装 TCP 连接
- 支持双向 TLS 认证
- 所有 Raft 节点间通信（心跳、日志复制、投票）都经过加密

### 4. 配置文件更新 (`config.example.yaml`)

添加了 TLS 配置节：

```yaml
# 传输层 TLS 加密配置 (P0-2 TLS 加密传输)
tls:
  enable_tls: false
  cert_path: "/etc/dbf/certs/server.crt"
  key_path: "/etc/dbf/certs/server.key"
  ca_path: "/etc/dbf/certs/ca.crt"
  min_version: "tls1.3"
  reload_interval: "0s"

# Raft 通信 TLS 配置 (P0-2 TLS 加密传输)
raft_tls:
  enable_tls: false
  cert_path: "/etc/dbf/certs/server.crt"
  key_path: "/etc/dbf/certs/server.key"
  ca_path: "/etc/dbf/certs/ca.crt"
  min_version: "tls1.3"
  reload_interval: "0s"
```

## 使用指南

### 1. 生成证书

在生产环境中，应使用受信任的 CA 签发证书。对于测试环境，可以使用自签名证书：

```bash
# 生成 CA 密钥和证书
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt \
  -subj "/CN=DBF CA/O=DBF/C=CN"

# 生成服务器密钥和证书签名请求
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/CN=node1.dbf.local/O=DBF/C=CN"

# 使用 CA 签署服务器证书
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -extfile <(echo "subjectAltName=DNS:node1.dbf.local,DNS:localhost,IP:127.0.0.1")
```

### 2. 启用 gRPC TLS

修改配置文件或在启动时指定参数：

```yaml
tls:
  enable_tls: true
  cert_path: "/path/to/server.crt"
  key_path: "/path/to/server.key"
  ca_path: "/path/to/ca.crt"
  min_version: "tls1.3"
```

或使用命令行：
```bash
./server --enable-tls \
  --tls-cert=/path/to/server.crt \
  --tls-key=/path/to/server.key \
  --tls-ca=/path/to/ca.crt
```

### 3. 启用 Raft TLS

```yaml
raft_tls:
  enable_tls: true
  cert_path: "/path/to/server.crt"
  key_path: "/path/to/server.key"
  ca_path: "/path/to/ca.crt"
  min_version: "tls1.3"
```

### 4. 证书热加载

要启用证书热加载（无需重启服务即可更新证书）：

```yaml
tls:
  enable_tls: true
  # ... 其他配置 ...
  reload_interval: "5m"  # 每 5 分钟检查并重新加载证书
```

## 安全建议

### 生产环境配置

1. **使用 TLS 1.3**: 仅提供 TLS 1.3，禁用旧版本
2. **强加密套件**: 仅使用 AEAD 加密套件
3. **双向认证**: 启用客户端证书验证
4. **证书管理**:
   - 使用受信任的 CA 签发证书
   - 定期轮换证书（建议 90 天）
   - 启用证书热加载以减少停机时间
5. **密钥保护**:
   - 私钥权限设置为 600
   - 使用密钥管理系统 (KMS) 存储密钥

### 证书轮换流程

1. 生成新证书（保留旧证书）
2. 部署新证书到所有节点
3. 等待证书热加载自动生效（或重启服务）
4. 验证所有节点通信正常
5. 撤销旧证书

## 测试

### 单元测试

```bash
# 测试 TLS 配置包
go test ./pkg/tls/... -v

# 测试 gRPC 服务器
go test ./internal/grpc/... -v

# 测试 Raft 节点
go test ./internal/raft/... -v
```

### 集成测试

创建测试证书并启动带 TLS 的服务器：

```bash
# 生成测试证书
mkdir -p /tmp/dbf-certs
cd /tmp/dbf-certs
# ... 使用上面的 OpenSSL 命令 ...

# 启动服务器
./server --port 50051 \
  --enable-tls \
  --tls-cert=/tmp/dbf-certs/server.crt \
  --tls-key=/tmp/dbf-certs/server.key \
  --tls-ca=/tmp/dbf-certs/ca.crt
```

## 监控和故障排除

### 日志

启用调试日志以查看 TLS 握手详情：

```yaml
logging:
  level: "debug"
```

### 常见问题

1. **证书验证失败**:
   - 检查证书路径是否正确
   - 验证证书是否由正确的 CA 签署
   - 检查证书是否过期

2. **TLS 握手失败**:
   - 确认客户端和服务器使用兼容的 TLS 版本
   - 检查加密套件是否匹配
   - 验证证书链是否完整

3. **Raft 节点无法连接**:
   - 确认所有节点都配置了 TLS
   - 检查防火墙是否允许 TLS 端口
   - 验证节点间证书是否相互信任

## 性能影响

TLS 加密会引入一定的性能开销：

- **CPU 使用**: 增加约 5-10%（TLS 1.3 优化后）
- **延迟**: 首次握手增加约 1-2ms
- **吞吐量**: 影响可忽略（TLS 1.3 0-RTT）

**优化建议**:
- 使用 TLS 1.3（比 TLS 1.2 更快）
- 启用会话复用
- 使用硬件加速（如支持）

## 兼容性

- **Go 版本**: 1.24+
- **TLS 版本**: TLS 1.2, TLS 1.3
- **gRPC**: 1.79.2+
- **HashiCorp Raft**: 1.7.3+

## 参考

- [Go TLS 文档](https://pkg.go.dev/crypto/tls)
- [gRPC TLS 认证](https://grpc.io/docs/guides/auth/)
- [TLS 1.3 RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446)
- [P0-1 gRPC 认证授权实现](../internal/grpc/auth.go)

## 变更历史

- **2026-03-12**: P0-2 TLS 加密传输初始实施
  - 创建 `pkg/tls/config.go`
  - 更新 `internal/grpc/server.go`
  - 更新 `internal/raft/node.go`
  - 添加单元测试
  - 编写使用文档
