# gRPC API 使用文档

本文档描述 Distributed Bloom Filter 的 gRPC API 使用方法。

## 服务地址

- **默认端口**: 8080
- **协议**: gRPC (HTTP/2)
- **示例地址**: `localhost:8080`

## 服务定义

### DBFService

提供 Bloom 过滤器的完整 gRPC API。

## RPC 方法

### 1. Add - 添加元素

添加单个元素到 Bloom 过滤器。

**请求**:
```protobuf
message AddRequest {
  bytes item = 1;  // 要添加的元素
}
```

**响应**:
```protobuf
message AddResponse {
  bool success = 1;  // 是否成功
  string error = 2;  // 错误信息（如果失败）
}
```

**示例**:
```bash
# 使用客户端添加元素
go run cmd/client/main.go -server localhost:8080 -action add -items "hello,world"
```

---

### 2. Remove - 删除元素

从 Bloom 过滤器删除单个元素。

**请求**:
```protobuf
message RemoveRequest {
  bytes item = 1;  // 要删除的元素
}
```

**响应**:
```protobuf
message RemoveResponse {
  bool success = 1;  // 是否成功
  string error = 2;  // 错误信息（如果失败）
}
```

**示例**:
```bash
go run cmd/client/main.go -server localhost:8080 -action remove -items "hello"
```

---

### 3. Contains - 查询元素

检查元素是否存在于 Bloom 过滤器。

**请求**:
```protobuf
message ContainsRequest {
  bytes item = 1;  // 要查询的元素
}
```

**响应**:
```protobuf
message ContainsResponse {
  bool exists = 1;  // 元素是否存在
  string error = 2;  // 错误信息
}
```

**示例**:
```bash
go run cmd/client/main.go -server localhost:8080 -action contains -items "hello,world"
```

---

### 4. BatchAdd - 批量添加

批量添加多个元素到 Bloom 过滤器。

**请求**:
```protobuf
message BatchAddRequest {
  repeated bytes items = 1;  // 要添加的元素列表
}
```

**响应**:
```protobuf
message BatchAddResponse {
  int32 success_count = 1;  // 成功添加的数量
  int32 failure_count = 2;  // 失败的数量
  repeated string errors = 3;  // 每个元素的错误信息
}
```

**示例**:
```bash
go run cmd/client/main.go -server localhost:8080 -action batch-add -items "item1,item2,item3"
```

---

### 5. BatchContains - 批量查询

批量查询多个元素是否存在。

**请求**:
```protobuf
message BatchContainsRequest {
  repeated bytes items = 1;  // 要查询的元素列表
}
```

**响应**:
```protobuf
message BatchContainsResponse {
  repeated bool results = 1;  // 每个元素的查询结果
  string error = 2;  // 错误信息
}
```

**示例**:
```bash
go run cmd/client/main.go -server localhost:8080 -action batch-contains -items "item1,item2,item3"
```

---

### 6. GetStats - 获取统计信息

获取节点和 Bloom 过滤器的统计信息。

**请求**:
```protobuf
message GetStatsRequest {
  // 无参数
}
```

**响应**:
```protobuf
message GetStatsResponse {
  string node_id = 1;     // 节点 ID
  bool is_leader = 2;     // 是否为 Raft leader
  string raft_state = 3;  // Raft 状态
  string leader = 4;      // Leader 地址
  int64 bloom_size = 5;   // Bloom 过滤器大小（bits）
  int32 bloom_k = 6;      // Hash 函数数量
  int64 bloom_count = 7;  // 近似元素数量
  int32 raft_port = 8;    // Raft 端口
  string error = 9;       // 错误信息
}
```

**示例**:
```bash
go run cmd/client/main.go -server localhost:8080 -action stats
```

---

## 启动服务器

```bash
cd cmd/server
go run main.go -port 8080 -raft-port 8081 -node-id node1 -bootstrap
```

### 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | 8080 | gRPC 服务器端口 |
| `-raft-port` | 8081 | Raft 共识端口 |
| `-data-dir` | ./data | 数据存储目录 |
| `-node-id` | node1 | 节点唯一标识 |
| `-k` | 3 | Hash 函数数量 |
| `-m` | 10000 | Bloom 过滤器大小（bits） |
| `-bootstrap` | false | 是否作为首个节点启动 |

---

## 客户端使用

### 基本用法

```bash
# 添加元素
go run cmd/client/main.go -server localhost:8080 -action add -items "myitem"

# 查询元素
go run cmd/client/main.go -server localhost:8080 -action contains -items "myitem"

# 批量操作
go run cmd/client/main.go -server localhost:8080 -action batch-add -items "item1,item2,item3"

# 查看统计
go run cmd/client/main.go -server localhost:8080 -action stats
```

### 可用操作

- `add`: 添加元素
- `remove`: 删除元素
- `contains`: 查询元素
- `batch-add`: 批量添加
- `batch-contains`: 批量查询
- `stats`: 获取统计信息

---

## 错误处理

所有 RPC 方法都返回错误信息字段。常见错误：

- `item cannot be empty`: 元素不能为空
- `failed to add item`: 添加失败（通常因为节点不是 leader）
- `failed to remove item`: 删除失败
- `node is not the leader`: 当前节点不是 Raft leader

---

## 测试

运行单元测试：

```bash
go test ./internal/grpc/... -v
```

测试覆盖：
- TestServerAdd: 添加操作测试
- TestServerContains: 查询操作测试
- TestServerBatchOperations: 批量操作测试
- TestServerRemove: 删除操作测试
- TestServerGetStats: 统计信息测试

---

## 注意事项

1. **Bloom 过滤器特性**: Contains 方法可能返回假阳性（false positive），但不会返回假阴性（false negative）
2. **Raft 共识**: Add 和 Remove 操作需要通过 Raft 共识，只有 leader 节点可以执行
3. **批量操作**: 批量操作中单个元素失败不会影响其他元素
4. **空元素**: 所有操作都不接受空元素

---

**文档版本**: 1.0  
**最后更新**: 2026-03-11
