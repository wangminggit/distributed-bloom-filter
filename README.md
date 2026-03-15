# Distributed Bloom Filter (DBF)

[![Go Report Card](https://goreportcard.com/badge/github.com/yourorg/dbf)](https://goreportcard.com/report/github.com/yourorg/dbf)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/github.com/yourorg/dbf.svg)](https://pkg.go.dev/github.com/yourorg/dbf)

**高性能、分布式、支持删除的布隆过滤器系统**

- 🚀 支持 10 万 + QPS
- 🗑️ 原生支持删除操作 (Counting Bloom Filter)
- 📦 Kubernetes 原生部署
- 🔐 数据持久化 + 多副本同步
- 📈 内置 Prometheus 监控

## 快速开始

```bash
# 安装
go get github.com/yourorg/dbf

# 单机运行
dbf server --mode=standalone

# Docker 运行
docker run -p 50051:50051 yourorg/dbf:latest

# Kubernetes 部署
kubectl apply -f deploy/k8s/
```

## 核心特性

| 特性 | 说明 |
|------|------|
| 分布式 | 一致性 Hash 分片，水平扩展 |
| 支持删除 | Counting Bloom Filter，计数器实现 |
| 持久化 | WAL + 定期快照，重启不丢数据 |
| 高可用 | 多副本同步，自动故障恢复 |
| 可观测 | Prometheus Metrics + 结构化日志 |

## 性能指标

| 指标 | 数值 |
|------|------|
| 数据容量 | 10 亿 + |
| QPS | 10 万 + |
| 误判率 | 0.1% (可配置) |
| P99 延迟 | < 5ms |
| 内存占用 | ~720MB (6 节点集群) |

## 架构概览

```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                    │
│                                                          │
│  ┌─────────────┐                                        │
│  │   Ingress   │  (gRPC/HTTP 负载均衡)                   │
│  └──────┬──────┘                                        │
│         │                                                │
│  ┌──────▼──────┐                                        │
│  │    API      │  (无状态服务，处理路由)                  │
│  │   Gateway   │                                        │
│  └──────┬──────┘                                        │
│         │                                                │
│  ┌────────┴────────┬────────┬────────┐                  │
│  │                 │        │        │                  │
│  ▼                 ▼        ▼        ▼                  │
│ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐ ┌────┐             │
│ │Node│ │Node│ │Node│ │Node│ │Node│ │Node│  (StatefulSet)│
│ │ 0  │ │ 1  │ │ 2  │ │ 3  │ │ 4  │ │ 5  │             │
│ └────┘ └────┘ └────┘ └────┘ └────┘ └────┘             │
│    │        │        │        │        │                │
│    └────────┴────────┴────────┴────────┘                │
│                     │                                    │
│            ┌────────▼────────┐                          │
│            │   etcd/PV       │  (元数据 + 持久化)         │
│            └─────────────────┘                          │
└─────────────────────────────────────────────────────────┘
```

## 使用示例

### Go Client

```go
package main

import (
    "github.com/yourorg/dbf/client"
)

func main() {
    // 连接集群
    c, _ := client.NewClient([]string{
        "dbf-gateway.default.svc.cluster.local:50051",
    })
    
    // 添加元素
    c.Add("user:12345")
    
    // 查询
    exists := c.Contains("user:12345")  // true
    
    // 删除
    c.Delete("user:12345")
    
    // 批量操作
    c.BatchAdd([]string{"a", "b", "c"})
    results := c.BatchContains([]string{"a", "x", "c"})  // [true, false, true]
}
```

### gRPC 调用

```protobuf
// 添加元素
rpc Add(AddRequest) returns (AddResponse);

// 删除元素
rpc Delete(DeleteRequest) returns (DeleteResponse);

// 查询是否存在
rpc Contains(ContainsRequest) returns (ContainsResponse);
```

## Java 示例

### Maven 依赖

```xml
<dependency>
    <groupId>io.grpc</groupId>
    <artifactId>grpc-netty-shaded</artifactId>
    <version>1.60.0</version>
</dependency>
<dependency>
    <groupId>io.grpc</groupId>
    <artifactId>grpc-protobuf</artifactId>
    <version>1.60.0</version>
</dependency>
<dependency>
    <groupId>io.grpc</groupId>
    <artifactId>grpc-stub</artifactId>
    <version>1.60.0</version>
</dependency>
```

### 快速开始

```java
import com.dbf.DBFClient;

public class Main {
    public static void main(String[] args) {
        DBFClient client = new DBFClient("localhost", 50051);
        
        // Add 操作
        client.add("user:12345");
        
        // Contains 操作
        boolean exists = client.contains("user:12345");
        System.out.println("Exists: " + exists);
        
        client.close();
    }
}
```

详细示例请查看 [examples/java/README.md](examples/java/README.md)

## 部署

### Docker

```bash
docker run -d \
  --name dbf \
  -p 50051:50051 \
  -v /data/dbf:/data \
  yourorg/dbf:latest
```

### Kubernetes

```bash
kubectl apply -f deploy/k8s/
```

详细部署文档见 [deploy/README.md](deploy/README.md)

## 监控

DBF 内置 Prometheus 指标：

```promql
# 操作 QPS
rate(dbf_operations_total[1m])

# P99 延迟
histogram_quantile(0.99, rate(dbf_operation_duration_seconds_bucket[1m]))

# 元素数量
dbf_element_count

# 内存使用
dbf_memory_usage_bytes
```

Grafana 仪表盘模板见 [deploy/grafana/dashboard.json](deploy/grafana/dashboard.json)

## 配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `capacity` | 1000000000 | 目标容量 |
| `false_positive_rate` | 0.001 | 误判率 |
| `replicas` | 3 | 副本数 |
| `shards` | 6 | 分片数 |
| `wal_enabled` | true | 启用 WAL |
| `snapshot_interval` | 5m | 快照间隔 |

完整配置见 [config.yaml](config.example.yaml)

## 开发

```bash
# 克隆
git clone https://github.com/yourorg/dbf.git
cd dbf

# 运行测试
go test ./...

# 构建
go build -o dbf ./cmd/server

# 运行
./dbf server --config=config.yaml
```

## 贡献

详见 [CONTRIBUTING.md](CONTRIBUTING.md)

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交变更 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 开启 Pull Request

## 许可证

Apache License 2.0 - 详见 [LICENSE](LICENSE)

## 致谢

- [Bloom Filter 原始论文](https://www.cs.cmu.edu/~dga/papers/bloom-cacm70.pdf)
- [Counting Bloom Filter](https://www.cs.cmu.edu/~dga/papers/counting-bloom.pdf)

---

**Star ⭐ 这个项目如果对你有帮助！**
