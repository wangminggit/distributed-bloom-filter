# 集成测试环境搭建指南

**作者**: David Wang  
**时间**: 2026-03-11  
**版本**: 1.0

---

## 🎯 目标

帮助 Sarah 快速搭建本地测试环境，启动 gRPC 服务器，并开始集成测试。

---

## 📋 前置条件

### 1. Go 环境

```bash
# 检查 Go 版本（需要 1.21+）
go version

# 如果未安装，请访问 https://go.dev/dl/
```

### 2. 项目依赖

```bash
cd /home/shequ/.openclaw/workspace/projects/distributed-bloom-filter

# 安装依赖
go mod download
```

---

## 🚀 启动 gRPC 服务器（单机模式）

### 方法 1: 直接运行（推荐用于测试）

```bash
cd /home/shequ/.openclaw/workspace/projects/distributed-bloom-filter

# 启动服务器（单机模式，无需集群）
go run cmd/server/main.go -port 8080 -raft-port 8081 -node-id node1 -bootstrap
```

**预期输出**:
```
gRPC server started on port 8080 (Raft: 8081)
Node 'node1' initialized with Bloom filter (m=10000, k=3)
```

### 方法 2: 使用编译后的二进制文件

```bash
# 编译服务器
cd cmd/server
go build -o ../../bin/dbf-server main.go

# 运行
../../bin/dbf-server -port 8080 -raft-port 8081 -node-id node1 -bootstrap
```

### 启动参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | 8080 | gRPC 服务器端口 |
| `-raft-port` | 8081 | Raft 共识端口（单机测试可忽略） |
| `-data-dir` | ./data | 数据存储目录 |
| `-node-id` | node1 | 节点 ID |
| `-bootstrap` | false | 是否作为首个节点启动（单机模式必须） |
| `-k` | 3 | Hash 函数数量 |
| `-m` | 10000 | Bloom 过滤器大小（bits） |

---

## ✅ 验证服务器连接

### 方法 1: 使用客户端 CLI

```bash
cd /home/shequ/.openclaw/workspace/projects/distributed-bloom-filter

# 添加测试元素
go run cmd/client/main.go -server localhost:8080 -action add -items "test1,test2,test3"

# 查询元素
go run cmd/client/main.go -server localhost:8080 -action contains -items "test1,test2,nonexistent"

# 查看统计信息
go run cmd/client/main.go -server localhost:8080 -action stats
```

### 方法 2: 使用 Go 测试代码

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    // 连接服务器
    conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()
    
    // 创建客户端
    client := pb.NewDBFServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // 测试 Add
    resp, err := client.Add(ctx, &pb.AddRequest{Item: []byte("hello")})
    if err != nil {
        log.Fatalf("Add failed: %v", err)
    }
    fmt.Printf("Add success: %v\n", resp.Success)
    
    // 测试 Contains
    containsResp, err := client.Contains(ctx, &pb.ContainsRequest{Item: []byte("hello")})
    if err != nil {
        log.Fatalf("Contains failed: %v", err)
    }
    fmt.Printf("Contains 'hello': %v\n", containsResp.Exists)
}
```

---

## 🧪 集成测试框架

### 测试文件位置

```
tests/integration/api_test.go
```

### 当前测试用例（需要实现）

1. **TestAddAndContains** - 验证 Add + Contains 操作
2. **TestAddAndRemove** - 验证 Add + Remove 操作
3. **TestBatchAdd** - 验证批量添加
4. **TestBatchContains** - 验证批量查询
5. **TestGetStats** - 验证统计接口

### 运行测试

```bash
cd tests

# 运行所有集成测试
make integration

# 或者直接使用 go test
go test ./integration/... -v
```

### 测试模板

```go
package integration

import (
    "context"
    "testing"
    "time"
    
    pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func TestAddAndContains(t *testing.T) {
    // 1. 连接服务器
    conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
    assert.NoError(t, err)
    defer conn.Close()
    
    client := pb.NewDBFServiceClient(conn)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // 2. 添加元素
    addResp, err := client.Add(ctx, &pb.AddRequest{Item: []byte("test-item")})
    assert.NoError(t, err)
    assert.True(t, addResp.Success)
    
    // 3. 验证元素存在
    containsResp, err := client.Contains(ctx, &pb.ContainsRequest{Item: []byte("test-item")})
    assert.NoError(t, err)
    assert.True(t, containsResp.Exists)
}
```

---

## 🔧 常见问题

### Q1: 端口被占用

**错误**: `bind: address already in use`

**解决**:
```bash
# 查找占用端口的进程
lsof -i :8080
lsof -i :8081

# 杀死进程
kill -9 <PID>

# 或者使用其他端口
go run cmd/server/main.go -port 8082 -raft-port 8083 -node-id node1 -bootstrap
```

### Q2: 找不到 proto 文件

**错误**: `cannot find package "github.com/wangminggit/distributed-bloom-filter/api/proto"`

**解决**:
```bash
# 确保在项目根目录
cd /home/shequ/.openclaw/workspace/projects/distributed-bloom-filter

# 重新生成 proto 文件（如果需要）
cd api/proto
protoc --go_out=. --go-grpc_out=. dbf.proto
```

### Q3: 依赖缺失

**错误**: `no required module provides package...`

**解决**:
```bash
go mod tidy
go mod download
```

---

## 📞 联系支持

- **负责人**: David Wang
- **沟通渠道**: Feishu
- **API 文档**: `docs/grpc-api.md`
- **测试文档**: `tests/README.md`

---

## ✅ 检查清单

开始测试前，请确认：

- [ ] Go 环境已安装（1.21+）
- [ ] 项目依赖已下载（`go mod download`）
- [ ] gRPC 服务器已启动（端口 8080）
- [ ] 客户端可以连接服务器
- [ ] 测试框架已准备（`tests/integration/api_test.go`）

---

**祝测试顺利！** 🚀
