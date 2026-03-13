# Distributed Bloom Filter (DBF)

[![Go Report Card](https://goreportcard.com/badge/github.com/wangminggit/distributed-bloom-filter)](https://goreportcard.com/report/github.com/wangminggit/distributed-bloom-filter)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/github.com/wangminggit/distributed-bloom-filter.svg)](https://pkg.go.dev/github.com/wangminggit/distributed-bloom-filter)
[![CI](https://github.com/wangminggit/distributed-bloom-filter/actions/workflows/ci.yml/badge.svg)](https://github.com/wangminggit/distributed-bloom-filter/actions/workflows/ci.yml)

**高性能、分布式、支持删除的布隆过滤器系统 - 生产就绪 v1.0.0**

- 🚀 支持 10 万 + QPS，P99 <5ms
- 🗑️ 原生支持删除操作 (Counting Bloom Filter)
- 🔐 企业级安全 (TLS/mTLS/认证/限流)
- 📦 分布式一致性 (Raft 共识算法)
- 💾 持久化存储 (WAL + 加密快照)
- 📈 Kubernetes 原生部署

---

## 🎯 项目状态

**当前版本**: v1.0.0-rc1 (即将发布)

**完成度**:
- ✅ M1: 核心数据结构
- ✅ M2: 分布式层 (Raft)
- ✅ M3: 持久化层
- ✅ M4: API 服务
- 🔄 M5: 测试收尾 (进行中)

**安全审计**: ✅ 11 个安全问题全部修复 (P0/P1/P2)

---

## 🚀 快速开始

### 安装

```bash
# 使用 Go 模块
go get github.com/wangminggit/distributed-bloom-filter

# 或克隆源码
git clone https://github.com/wangminggit/distributed-bloom-filter.git
cd distributed-bloom-filter
go build -o dbf ./cmd/server
```

### 单机模式

```bash
# 启动服务器
./dbf server --mode=standalone --port=7000

# 或使用配置文件
./dbf server --config=config.yaml
```

### Docker

```bash
# 构建镜像
docker build -t dbf:latest .

# 运行容器
docker run -d \
  --name dbf \
  -p 7000:7000 \
  -p 8080:8080 \
  -v /data/dbf:/data \
  dbf:latest
```

### Kubernetes 集群

```bash
# 部署 5 节点集群
kubectl apply -f deploy/k8s/

# 查看状态
kubectl get pods -l app=dbf
```

---

## 📦 核心特性

| 特性 | 说明 | 状态 |
|------|------|------|
| **分布式架构** | Raft 共识算法，5 节点集群容错 2 节点 | ✅ |
| **支持删除** | Counting Bloom Filter，4 位计数器 | ✅ |
| **数据持久化** | WAL 日志 + AES-256-GCM 加密快照 | ✅ |
| **高可用性** | Leader 自动选举，故障恢复 <500ms | ✅ |
| **安全防护** | mTLS 双向认证，HMAC-SHA256 签名，限流 | ✅ |
| **多协议支持** | gRPC + HTTP RESTful API | ✅ |
| **Go SDK** | 自动重试，Leader 发现，连接池 | ✅ |
| **可观测性** | 结构化日志，Prometheus 指标 | 🔄 |

---

## 📊 性能指标

| 指标 | 数值 | 测试条件 |
|------|------|----------|
| **吞吐量** | 10 万 + QPS | 5 节点集群，批量操作 |
| **P99 延迟** | <5ms | 单机模式 |
| **误判率** | 0.1% | 默认配置，10 亿容量 |
| **内存占用** | ~720MB | 6 节点集群，10 亿容量 |
| **故障恢复** | <500ms | Leader 切换 |

*详细性能报告见 [.learnings/PERFORMANCE-REPORT.md](.learnings/PERFORMANCE-REPORT.md)*

---

## 💻 使用示例

### Go SDK

```go
package main

import (
    "fmt"
    "log"
    "github.com/wangminggit/distributed-bloom-filter/sdk"
)

func main() {
    // 初始化客户端（自动发现 Leader）
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

    // 添加元素
    if err := client.Add("user:12345"); err != nil {
        log.Fatal(err)
    }

    // 查询是否存在
    exists, err := client.Contains("user:12345")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Exists: %v\n", exists) // true

    // 删除元素
    if err := client.Remove("user:12345"); err != nil {
        log.Fatal(err)
    }

    // 批量操作
    keys := []string{"a", "b", "c", "d"}
    if err := client.BatchAdd(keys); err != nil {
        log.Fatal(err)
    }

    results, err := client.BatchContains(keys)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Batch results: %v\n", results) // map[a:true b:true c:true d:true]

    // 获取节点状态
    stats, err := client.GetStats()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Cluster stats: %+v\n", stats)
}
```

### HTTP API

```bash
# 添加元素
curl -X POST http://localhost:8080/api/v1/add \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"key": "user:12345"}'

# 查询元素
curl -X POST http://localhost:8080/api/v1/contains \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"key": "user:12345"}'

# 批量添加
curl -X POST http://localhost:8080/api/v1/batch/add \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{"keys": ["a", "b", "c"]}'

# 获取集群状态
curl http://localhost:8080/api/v1/cluster \
  -H "X-API-Key: your-api-key"
```

### gRPC 调用

```go
package main

import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
)

func main() {
    // TLS 连接
    creds, _ := credentials.NewClientTLSFromFile("cert.pem", "")
    conn, _ := grpc.Dial("localhost:7000", grpc.WithTransportCredentials(creds))
    defer conn.Close()

    client := pb.NewDBFClient(conn)

    // 添加元素
    resp, _ := client.Add(context.Background(), &pb.AddRequest{
        Key: "user:12345",
        Auth: &pb.AuthMetadata{
            ApiKey: "your-api-key",
            Timestamp: time.Now().Unix(),
            Signature: "hmac-sha256-signature",
        },
    })

    fmt.Printf("Added: %v\n", resp.Success)
}
```

更多示例见 [examples/](examples/) 目录。

---

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                    │
│                                                          │
│  ┌─────────────┐                                        │
│  │   Ingress   │  (gRPC/HTTP 负载均衡 + TLS 终止)          │
│  └──────┬──────┘                                        │
│         │                                                │
│  ┌──────▼──────┐                                        │
│  │    API      │  (HTTP Gateway, 认证/限流)               │
│  │   Gateway   │                                        │
│  └──────┬──────┘                                        │
│         │                                                │
│  ┌────────┴────────┬────────┬────────┐                  │
│  │                 │        │        │                  │
│  ▼                 ▼        ▼        ▼                  │
│ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐                     │
│ │Node│ │Node│ │Node│ │Node│ │Node│  (Raft 集群)          │
│ │ 0  │ │ 1  │ │ 2  │ │ 3  │ │ 4  │                     │
│ │Ldr │ │Flw │ │Flw │ │Flw │ │Flw │                     │
│ └─┬──┘ └─┬──┘ └─┬──┘ └─┬──┘ └─┬──┘                     │
│   │        │        │        │        │                  │
│   └────────┴────────┴────────┴────────┘                  │
│                     │                                    │
│            ┌────────▼────────┐                          │
│            │   BoltDB + WAL  │  (持久化存储)              │
│            └─────────────────┘                          │
└─────────────────────────────────────────────────────────┘
```

**核心组件**:
- **Raft 共识层**: Leader 选举、日志复制、故障恢复
- **Bloom Filter 层**: Counting Bloom Filter 核心算法
- **持久化层**: WAL 日志 + AES-256-GCM 加密快照
- **API 层**: gRPC 服务 + HTTP Gateway + Go SDK

详细架构文档见 [ARCHITECTURE.md](ARCHITECTURE.md)。

---

## 🔐 安全特性

### 认证与授权
- **mTLS 双向认证**: 客户端和服务器双向证书验证
- **API Key 认证**: HMAC-SHA256 签名 + 时间戳验证
- **重放攻击防护**: 时间戳窗口 + 请求 ID 缓存

### 数据加密
- **传输加密**: TLS 1.3 加密传输
- **静态加密**: AES-256-GCM 加密快照
- **密钥管理**: 密钥轮换 + 安全缓存

### 限流防护
- **请求限流**: 令牌桶算法，默认 100 请求/秒
- **连接限流**: 最大并发连接数限制
- **DoS 防护**: 异常流量检测和阻断

详细安全策略见 [SECURITY.md](SECURITY.md)。

---

## 📁 项目结构

```
distributed-bloom-filter/
├── api/                    # API 定义
│   └── proto/             # Protocol Buffers
├── cmd/                    # 可执行文件
│   └── server/            # 服务器入口
├── internal/               # 内部实现
│   ├── bloom/             # Bloom Filter 核心
│   ├── raft/              # Raft 共识层
│   ├── wal/               # WAL 持久化
│   ├── grpc/              # gRPC 服务
│   ├── gateway/           # HTTP Gateway
│   └── metadata/          # 元数据服务
├── pkg/                    # 公共包
│   └── bloom/             # 可复用 Bloom Filter
├── sdk/                    # Go SDK
├── examples/               # 使用示例
├── deploy/                 # 部署配置
│   └── k8s/               # Kubernetes 配置
├── tests/                  # 集成测试
├── .learnings/             # 设计和完成报告
├── go.mod                  # Go 模块定义
├── SECURITY.md             # 安全策略
└── README.md               # 本文件
```

详细项目结构说明见 [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)。

---

## 🧪 测试

```bash
# 运行所有测试
go test -race ./...

# 运行覆盖率测试
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行性能基准测试
go test -bench=. -benchmem ./pkg/bloom/

# 运行压力测试
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/contains
```

**测试覆盖率**: 目标 >80% (当前进行中)

**测试报告**: [.learnings/TEST-FINAL.md](.learnings/TEST-FINAL.md)

---

## 📝 配置

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DBF_MODE` | standalone | 运行模式 (standalone/cluster) |
| `DBF_PORT` | 7000 | gRPC 端口 |
| `DBF_HTTP_PORT` | 8080 | HTTP 端口 |
| `DBF_API_KEY` | - | API 密钥 |
| `DBF_TLS_CERT` | - | TLS 证书路径 |
| `DBF_TLS_KEY` | - | TLS 密钥路径 |

### 配置文件 (config.yaml)

```yaml
server:
  mode: cluster
  port: 7000
  httpPort: 8080
  
cluster:
  nodes:
    - host: node-0
      port: 7000
    - host: node-1
      port: 7000
    - host: node-2
      port: 7000
    - host: node-3
      port: 7000
    - host: node-4
      port: 7000
  raft:
    electionTimeout: 200ms
    heartbeatInterval: 50ms

security:
  apiKey: your-api-key
  tls:
    enabled: true
    certFile: /certs/server.crt
    keyFile: /certs/server.key
  rateLimit:
    enabled: true
    requestsPerSecond: 100

bloom:
  capacity: 1000000000
  falsePositiveRate: 0.001

persistence:
  wal:
    enabled: true
    dir: /data/wal
  snapshot:
    enabled: true
    dir: /data/snapshot
    interval: 5m
```

完整配置示例见 [config.example.yaml](config.example.yaml)。

---

## 📚 文档

| 文档 | 说明 |
|------|------|
| [ARCHITECTURE.md](ARCHITECTURE.md) | 系统架构设计 |
| [API.md](docs/API.md) | API 参考文档 |
| [DEPLOYMENT.md](docs/DEPLOYMENT.md) | 部署指南 |
| [CONFIGURATION.md](docs/CONFIGURATION.md) | 配置说明 |
| [SECURITY.md](SECURITY.md) | 安全策略 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献指南 |
| [CHANGELOG.md](CHANGELOG.md) | 变更日志 |

---

## 🤝 贡献

欢迎贡献代码、报告问题或提出建议！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交变更 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

详见 [CONTRIBUTING.md](CONTRIBUTING.md)。

---

## 📄 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE)

---

## 🙏 致谢

- [Bloom Filter 原始论文](https://www.cs.cmu.edu/~dga/papers/bloom-cacm70.pdf)
- [Counting Bloom Filter](https://www.cs.cmu.edu/~dga/papers/counting-bloom.pdf)
- [HashiCorp Raft](https://github.com/hashicorp/raft)
- [gRPC](https://grpc.io/)

---

**Star ⭐ 这个项目如果对你有帮助！**

**Issues & PRs**: https://github.com/wangminggit/distributed-bloom-filter

---

*Last updated: 2026-03-13*
