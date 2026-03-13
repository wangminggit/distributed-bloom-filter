# API 参考文档

**版本**: v1.0.0-rc1  
**最后更新**: 2026-03-13

---

## 📋 目录

1. [gRPC API](#grpc-api)
2. [HTTP RESTful API](#http-restful-api)
3. [Go SDK](#go-sdk)
4. [错误码](#错误码)
5. [认证说明](#认证说明)

---

## gRPC API

### 服务定义

```protobuf
service DBF {
  // 添加元素
  rpc Add(AddRequest) returns (AddResponse);
  
  // 删除元素
  rpc Remove(RemoveRequest) returns (RemoveResponse);
  
  // 查询元素
  rpc Contains(ContainsRequest) returns (ContainsResponse);
  
  // 批量添加
  rpc BatchAdd(BatchAddRequest) returns (BatchResponse);
  
  // 批量查询
  rpc BatchContains(BatchContainsRequest) returns (BatchContainsResponse);
  
  // 获取统计信息
  rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
}
```

### 消息类型

#### AddRequest

```protobuf
message AddRequest {
  string key = 1;           // 要添加的元素
  AuthMetadata auth = 2;    // 认证信息
}

message AuthMetadata {
  string api_key = 1;       // API 密钥
  int64 timestamp = 2;      // Unix 时间戳 (秒)
  string signature = 3;     // HMAC-SHA256 签名
}
```

#### AddResponse

```protobuf
message AddResponse {
  bool success = 1;         // 操作是否成功
  string error = 2;         // 错误信息 (如果有)
  string leader = 3;        // 当前 Leader 地址 (如果不是 Leader)
}
```

#### ContainsRequest

```protobuf
message ContainsRequest {
  string key = 1;
  AuthMetadata auth = 2;
}
```

#### ContainsResponse

```protobuf
message ContainsResponse {
  bool exists = 1;          // 元素是否存在
  string error = 2;
  string leader = 3;
}
```

#### BatchAddRequest

```protobuf
message BatchAddRequest {
  repeated string keys = 1; // 批量添加的元素列表
  AuthMetadata auth = 2;
}
```

#### BatchResponse

```protobuf
message BatchResponse {
  bool success = 1;
  int32 added_count = 2;    // 成功添加的数量
  repeated string errors = 3; // 错误列表
  string leader = 4;
}
```

#### BatchContainsRequest

```protobuf
message BatchContainsRequest {
  repeated string keys = 1;
  AuthMetadata auth = 2;
}
```

#### BatchContainsResponse

```protobuf
message BatchContainsResponse {
  map<string, bool> results = 1; // key -> exists 映射
  string error = 2;
  string leader = 3;
}
```

#### GetStatsRequest / GetStatsResponse

```protobuf
message GetStatsRequest {
  AuthMetadata auth = 1;
}

message GetStatsResponse {
  int64 element_count = 1;      // 元素总数
  int64 capacity = 2;           // 容量
  double false_positive_rate = 3; // 误判率
  string leader = 4;
  ClusterInfo cluster = 5;      // 集群信息
}

message ClusterInfo {
  int32 node_count = 1;         // 节点数
  string leader_addr = 2;       // Leader 地址
  repeated string follower_addrs = 3; // Follower 地址列表
}
```

---

## HTTP RESTful API

**基础 URL**: `http://localhost:8080/api/v1`

### 认证

所有请求需要在 Header 中包含 API Key:

```
X-API-Key: your-api-key
X-Timestamp: 1710316800
X-Signature: hmac-sha256-signature
```

### 端点

#### POST /add

添加单个元素。

**请求**:
```json
{
  "key": "user:12345"
}
```

**响应** (200 OK):
```json
{
  "success": true,
  "error": null,
  "leader": "node-0:7000"
}
```

**错误响应** (400/503):
```json
{
  "success": false,
  "error": "Invalid API key",
  "leader": "node-0:7000"
}
```

---

#### DELETE /remove

删除单个元素。

**请求**:
```json
{
  "key": "user:12345"
}
```

**响应** (200 OK):
```json
{
  "success": true,
  "error": null,
  "leader": "node-0:7000"
}
```

---

#### POST /contains

查询元素是否存在。

**请求**:
```json
{
  "key": "user:12345"
}
```

**响应** (200 OK):
```json
{
  "exists": true,
  "error": null,
  "leader": "node-0:7000"
}
```

---

#### POST /batch/add

批量添加元素。

**请求**:
```json
{
  "keys": ["a", "b", "c", "d"]
}
```

**响应** (200 OK):
```json
{
  "success": true,
  "added_count": 4,
  "errors": [],
  "leader": "node-0:7000"
}
```

---

#### POST /batch/contains

批量查询元素。

**请求**:
```json
{
  "keys": ["a", "x", "c"]
}
```

**响应** (200 OK):
```json
{
  "results": {
    "a": true,
    "x": false,
    "c": true
  },
  "error": null,
  "leader": "node-0:7000"
}
```

---

#### GET /status

获取当前节点状态。

**响应** (200 OK):
```json
{
  "node_id": "node-0",
  "state": "Leader",
  "uptime": 3600,
  "element_count": 1000000,
  "memory_usage_bytes": 150000000
}
```

---

#### GET /cluster

获取集群信息。

**响应** (200 OK):
```json
{
  "node_count": 5,
  "leader_addr": "node-0:7000",
  "follower_addrs": [
    "node-1:7000",
    "node-2:7000",
    "node-3:7000",
    "node-4:7000"
  ],
  "replication_lag_ms": 10
}
```

---

## Go SDK

### 初始化

```go
import "github.com/wangminggit/distributed-bloom-filter/sdk"

// 基础配置
client, err := sdk.NewClient(sdk.ClientConfig{
    Addresses: []string{
        "localhost:7000",
        "localhost:7001",
        "localhost:7002",
    },
    APIKey: "your-api-key",
    Timeout: 5 * time.Second,
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

### 高级配置

```go
client, err := sdk.NewClient(sdk.ClientConfig{
    Addresses: []string{"node-0:7000", "node-1:7000"},
    APIKey: "your-api-key",
    
    // TLS 配置
    TLS: sdk.TLSConfig{
        Enabled: true,
        CertFile: "/path/to/client.crt",
        KeyFile: "/path/to/client.key",
        CAFile: "/path/to/ca.crt",
    },
    
    // 重试配置
    Retry: sdk.RetryConfig{
        MaxRetries: 3,
        InitialDelay: 100 * time.Millisecond,
        MaxDelay: 5 * time.Second,
        Multiplier: 2.0,
    },
    
    // 连接池配置
    Pool: sdk.PoolConfig{
        MaxIdleConns: 10,
        MaxOpenConns: 100,
        ConnMaxLifetime: 5 * time.Minute,
    },
})
```

### API 方法

#### Add

```go
err := client.Add("user:12345")
if err != nil {
    log.Fatal(err)
}
```

#### Remove

```go
err := client.Remove("user:12345")
if err != nil {
    log.Fatal(err)
}
```

#### Contains

```go
exists, err := client.Contains("user:12345")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Exists: %v\n", exists)
```

#### BatchAdd

```go
keys := []string{"a", "b", "c"}
err := client.BatchAdd(keys)
if err != nil {
    log.Fatal(err)
}
```

#### BatchContains

```go
keys := []string{"a", "x", "c"}
results, err := client.BatchContains(keys)
if err != nil {
    log.Fatal(err)
}
// results: map[string]bool{"a": true, "x": false, "c": true}
```

#### GetStats

```go
stats, err := client.GetStats()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Element count: %d\n", stats.ElementCount)
fmt.Printf("Cluster nodes: %d\n", stats.Cluster.NodeCount)
```

---

## 错误码

### HTTP 错误码

| 状态码 | 说明 |
|--------|------|
| 200 OK | 请求成功 |
| 400 Bad Request | 请求参数错误 |
| 401 Unauthorized | 认证失败 (API Key 无效/签名错误) |
| 403 Forbidden | 权限不足 |
| 408 Request Timeout | 请求超时 |
| 429 Too Many Requests | 请求频率超限 |
| 500 Internal Server Error | 服务器内部错误 |
| 503 Service Unavailable | 服务不可用 (非 Leader 节点) |

### gRPC 错误码

| 错误码 | 说明 |
|--------|------|
| OK (0) | 成功 |
| INVALID_ARGUMENT (3) | 参数错误 |
| UNAUTHENTICATED (16) | 认证失败 |
| PERMISSION_DENIED (7) | 权限不足 |
| RESOURCE_EXHAUSTED (8) | 资源耗尽 (限流) |
| UNAVAILABLE (14) | 服务不可用 |
| INTERNAL (13) | 内部错误 |

### 业务错误

| 错误信息 | 说明 |
|----------|------|
| `Invalid API key` | API Key 无效 |
| `Invalid signature` | 签名验证失败 |
| `Timestamp expired` | 时间戳过期 (超过 5 分钟) |
| `Rate limit exceeded` | 超过速率限制 |
| `Not leader, redirect to <addr>` | 当前节点不是 Leader |
| `Key too long` | Key 长度超过限制 (最大 1KB) |
| `Batch size too large` | 批量操作数量过大 (最大 1000) |

---

## 认证说明

### HMAC-SHA256 签名生成

```python
import hmac
import hashlib
import time

api_key = "your-api-key"
timestamp = str(int(time.time()))

# 签名字符串：timestamp + api_key
message = f"{timestamp}{api_key}"

# 生成 HMAC-SHA256 签名
signature = hmac.new(
    api_key.encode('utf-8'),
    message.encode('utf-8'),
    hashlib.sha256
).hexdigest()

# 请求头
headers = {
    "X-API-Key": api_key,
    "X-Timestamp": timestamp,
    "X-Signature": signature,
}
```

### Go 示例

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "strconv"
    "time"
)

func generateSignature(apiKey string) (timestamp, signature string) {
    ts := strconv.FormatInt(time.Now().Unix(), 10)
    
    h := hmac.New(sha256.New, []byte(apiKey))
    h.Write([]byte(ts + apiKey))
    sig := hex.EncodeToString(h.Sum(nil))
    
    return ts, sig
}
```

### 时间戳验证

- 服务器会验证请求时间戳与当前时间的差值
- 默认允许的时间窗口：**5 分钟**
- 超过时间窗口的请求会被拒绝，防止重放攻击

---

## 限流说明

### 默认限制

| 限制项 | 默认值 | 说明 |
|--------|--------|------|
| 请求速率 | 100 请求/秒 | 每 IP/API Key |
| 并发连接 | 1000 | 每节点 |
| 批量大小 | 1000 | 每次批量操作最大元素数 |
| Key 长度 | 1KB | 单个 Key 最大长度 |

### 自定义限流

在配置文件中调整:

```yaml
security:
  rateLimit:
    enabled: true
    requestsPerSecond: 200  # 自定义速率
    burstSize: 500          # 突发容量
```

---

## 最佳实践

### 1. 批量操作

优先使用批量 API 减少网络往返:

```go
// ❌ 不推荐：多次网络请求
for _, key := range keys {
    client.Contains(key)
}

// ✅ 推荐：一次批量请求
results, _ := client.BatchContains(keys)
```

### 2. 连接复用

复用客户端实例，避免频繁创建连接:

```go
// ❌ 不推荐：每次请求创建新客户端
func query(key string) bool {
    client, _ := sdk.NewClient(config)
    defer client.Close()
    exists, _ := client.Contains(key)
    return exists
}

// ✅ 推荐：全局单例
var globalClient *sdk.Client
func init() {
    globalClient, _ = sdk.NewClient(config)
}
```

### 3. 错误处理

正确处理 Leader 重定向:

```go
result, err := client.Contains(key)
if err != nil {
    if strings.Contains(err.Error(), "Not leader") {
        // SDK 会自动重试并切换到新 Leader
        // 无需手动处理
    }
}
```

### 4. 超时设置

根据业务场景设置合适的超时:

```go
client, _ := sdk.NewClient(sdk.ClientConfig{
    Timeout: 5 * time.Second,  // 默认 5 秒
    // 读操作可以设置更短超时
    ReadTimeout: 2 * time.Second,
    // 写操作可以设置更长超时
    WriteTimeout: 10 * time.Second,
})
```

---

**API 更新日志**: 详见 [CHANGELOG.md](../CHANGELOG.md)

**问题反馈**: https://github.com/wangminggit/distributed-bloom-filter/issues
