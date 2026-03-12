# P0-1 gRPC 认证授权实现文档

## 概述

本模块实现了 DBF 项目的 gRPC 认证授权功能，支持两种认证方式：
1. **mTLS 双向认证** - 基于证书的双向 TLS 认证
2. **JWT Token 认证** - 基于 JWT 的 Token 认证

## 技术实现

### 1. mTLS 双向认证

**工作原理**:
- 服务器和客户端都需要提供有效的 X.509 证书
- 证书由自签名 CA 签发
- 使用 TLS 1.2+ 加密通信

**证书结构**:
```
certs/
├── ca.crt          # CA 证书（公钥）
├── ca.key          # CA 私钥（保密）
├── server.crt      # 服务器证书
├── server.key      # 服务器私钥
├── client.crt      # 客户端证书
└── client.key      # 客户端私钥
```

### 2. JWT Token 认证

**Token 格式**:
```json
{
  "node_id": "node1",
  "permissions": ["read", "write"],
  "exp": 1234567890,
  "iat": 1234567800,
  "iss": "dbf-auth"
}
```

**认证流程**:
1. 客户端从 metadata 中提取 `authorization: Bearer <token>`
2. 验证 Token 签名和过期时间
3. 验证 node_id 和权限

## 使用方法

### 生成证书

```bash
# 生成自签名证书
./scripts/generate-certs.sh ./certs
```

### 启动服务器（mTLS）

```bash
./server \
  --port 8080 \
  --raft-port 8081 \
  --node-id node1 \
  --enable-mtls \
  --ca-cert ./certs/ca.crt \
  --server-cert ./certs/server.crt \
  --server-key ./certs/server.key
```

### 启动服务器（Token 认证）

```bash
./server \
  --port 8080 \
  --raft-port 8081 \
  --node-id node1 \
  --enable-token-auth \
  --jwt-secret "your-secret-key-at-least-32-bytes" \
  --token-expiry 24h
```

### 配置文件方式

编辑 `config.yaml`:

```yaml
auth:
  enable_mtls: true
  ca_cert_path: "/etc/dbf/certs/ca.crt"
  server_cert_path: "/etc/dbf/certs/server.crt"
  server_key_path: "/etc/dbf/certs/server.key"
  
  enable_token_auth: false
  jwt_secret_key: ""
  token_expiry: "24h"
```

## 客户端集成

### Go 客户端（mTLS）

```go
import (
    "crypto/tls"
    "crypto/x509"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
)

// 加载客户端证书
cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
if err != nil {
    log.Fatal(err)
}

// 加载 CA 证书
caCert, err := os.ReadFile("ca.crt")
if err != nil {
    log.Fatal(err)
}
caPool := x509.NewCertPool()
caPool.AppendCertsFromPEM(caCert)

// 创建 TLS 配置
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    RootCAs:      caPool,
}

// 创建 gRPC 连接
conn, err := grpc.Dial(
    "localhost:8080",
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
)
```

### Go 客户端（Token 认证）

```go
import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/metadata"
)

// 生成 Token
token, err := grpc.GenerateToken(&AuthConfig{
    JWTSecretKey: "your-secret-key",
    TokenExpiry:  time.Hour,
}, "node1", []string{"read", "write"})

// 创建带 Token 的上下文
ctx := metadata.NewOutgoingContext(
    context.Background(),
    metadata.MD{"authorization": []string{"Bearer " + token}},
)

// 使用上下文调用 gRPC 方法
client.Add(ctx, &proto.AddRequest{Item: []byte("test")})
```

## 安全建议

### 生产环境部署

1. **使用正式 CA 证书**
   - 不要使用自签名证书
   - 从可信 CA 购买证书

2. **保护私钥**
   - 设置文件权限：`chmod 600 *.key`
   - 使用密钥管理服务（如 AWS KMS、HashiCorp Vault）

3. **JWT 密钥管理**
   - 使用强随机密钥（至少 32 字节）
   - 定期轮换密钥
   - 通过环境变量或密钥管理服务传递

4. **证书轮换**
   - 设置合理的证书有效期
   - 实现自动证书轮换机制

### 测试环境

```bash
# 快速生成测试证书（有效期 365 天）
./scripts/generate-certs.sh ./test-certs

# 启动测试服务器
./server --enable-mtls \
  --ca-cert ./test-certs/ca.crt \
  --server-cert ./test-certs/server.crt \
  --server-key ./test-certs/server.key
```

## 测试

### 单元测试

```bash
# 运行认证模块测试
go test -v ./internal/grpc/... -run "TestAuth"

# 运行所有测试
go test ./internal/grpc/...
```

### 集成测试

```bash
# 生成测试证书
./scripts/generate-certs.sh ./test-certs

# 启动服务器
./server --enable-mtls \
  --ca-cert ./test-certs/ca.crt \
  --server-cert ./test-certs/server.crt \
  --server-key ./test-certs/server.key &

# 使用测试客户端连接
./client --tls \
  --ca-cert ./test-certs/ca.crt \
  --client-cert ./test-certs/client.crt \
  --client-key ./test-certs/client.key
```

## 错误处理

### 常见错误

| 错误 | 原因 | 解决方案 |
|------|------|----------|
| `client certificate required` | 客户端未提供证书 | 配置客户端证书 |
| `invalid client certificate` | 证书验证失败 | 检查证书是否由正确 CA 签发 |
| `token expired` | Token 已过期 | 重新生成 Token |
| `missing authorization header` | 缺少认证头 | 添加 `authorization: Bearer <token>` |

## 性能影响

- **mTLS**: 握手阶段增加约 1-2ms 延迟（证书验证）
- **Token 认证**: 每次请求增加约 0.1ms（JWT 验证）
- **建议**: 使用连接池减少 TLS 握手开销

## 后续改进

- [ ] 支持 OAuth2/OIDC 集成
- [ ] 实现证书自动轮换
- [ ] 添加认证指标监控
- [ ] 支持细粒度权限控制（RBAC）

## 相关文件

- `internal/grpc/auth.go` - 认证核心逻辑
- `internal/grpc/auth_test.go` - 单元测试
- `internal/grpc/server.go` - gRPC 服务器（集成认证）
- `cmd/server/main.go` - 服务器启动（证书加载）
- `scripts/generate-certs.sh` - 证书生成脚本
- `config.example.yaml` - 配置示例
