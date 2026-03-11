# Project Structure

```
distributed-bloom-filter/
├── README.md                 # 项目说明文档
├── ARCHITECTURE.md           # 架构设计文档
├── CONTRIBUTING.md           # 贡献指南
├── CHANGELOG.md              # 变更日志
├── LICENSE                   # Apache 2.0 许可证
├── go.mod                    # Go 模块定义
├── go.sum                    # Go 依赖校验
├── Makefile                  # 构建脚本
├── config.example.yaml       # 配置示例
│
├── cmd/
│   ├── server/
│   │   └── main.go           # 服务器入口
│   └── client/
│       └── main.go           # CLI 客户端
│
├── pkg/
│   ├── bloom/
│   │   ├── counting.go       # Counting Bloom Filter 核心实现
│   │   ├── hash.go           # 哈希函数 (double hashing)
│   │   └── bloom_test.go     # 单元测试
│   │
│   ├── raft/
│   │   ├── node.go           # Raft 节点实现
│   │   ├── log.go            # 日志复制
│   │   ├── election.go       # Leader 选举
│   │   └── fsm.go            # 状态机
│   │
│   ├── storage/
│   │   ├── wal.go            # Write-Ahead Log
│   │   ├── snapshot.go       # 快照管理
│   │   └── recovery.go       # 数据恢复
│   │
│   ├── cluster/
│   │   ├── shard.go          # 分片管理
│   │   ├── replication.go    # 副本同步
│   │   └── discovery.go      # 服务发现
│   │
│   └── metrics/
│       └── prometheus.go     # Prometheus 指标
│
├── api/
│   ├── proto/
│   │   └── dbf.proto         # gRPC 协议定义
│   └── grpc/
│       ├── server.go         # gRPC 服务器
│       └── client.go         # gRPC 客户端
│
├── deploy/
│   ├── k8s/
│   │   ├── statefulset.yaml  # Storage Node StatefulSet
│   │   ├── deployment.yaml   # API Gateway Deployment
│   │   ├── service.yaml      # Services
│   │   ├── configmap.yaml    # 配置
│   │   ├── pvc.yaml          # 持久化卷
│   │   └── README.md         # K8s 部署说明
│   │
│   ├── docker/
│   │   └── Dockerfile        # Docker 镜像构建
│   │
│   └── grafana/
│       └── dashboard.json    # Grafana 仪表盘
│
├── configs/
│   ├── default.yaml          # 默认配置
│   └── production.yaml       # 生产环境配置
│
├── scripts/
│   ├── benchmark.sh          # 性能测试脚本
│   ├── setup-cluster.sh      # 集群搭建脚本
│   └── backup.sh             # 备份脚本
│
├── tests/
│   ├── unit/                 # 单元测试
│   ├── integration/          # 集成测试
│   └── e2e/                  # 端到端测试
│
└── docs/
    ├── api.md                # API 文档
    ├── deployment.md         # 部署指南
    ├── monitoring.md         # 监控指南
    ├── tuning.md             # 性能调优
    └── faq.md                # 常见问题
```

## 目录说明

### `/cmd`
可执行命令的入口文件。
- `server/` - 主服务器进程
- `client/` - CLI 客户端工具

### `/pkg`
核心业务逻辑库代码，可被外部项目导入使用。
- `bloom/` - Counting Bloom Filter 实现
- `raft/` - Raft 共识算法实现
- `storage/` - 持久化层 (WAL + 快照)
- `cluster/` - 集群管理 (分片、复制、发现)
- `metrics/` - 监控指标

### `/api`
API 相关定义。
- `proto/` - Protocol Buffers 定义
- `grpc/` - gRPC 服务实现

### `/deploy`
部署相关文件。
- `k8s/` - Kubernetes 部署清单
- `docker/` - Docker 镜像构建
- `grafana/` - 监控仪表盘

### `/configs`
配置文件模板。

### `/scripts`
辅助脚本。

### `/tests`
测试代码。
- `unit/` - 单元测试
- `integration/` - 集成测试（需要启动集群）
- `e2e/` - 端到端测试

### `/docs`
详细文档。

## 核心模块依赖关系

```
cmd/server
    │
    ├── api/grpc/server
    │       │
    │       └── pkg/cluster/shard
    │               │
    │               ├── pkg/raft/node
    │               │       │
    │               │       └── pkg/storage/wal
    │               │       └── pkg/storage/snapshot
    │               │
    │               └── pkg/bloom/counting
    │
    └── pkg/metrics/prometheus
```

## 添加新功能

按照 Go 项目标准布局，新功能应该：

1. **核心逻辑** → `/pkg/<module>/`
2. **API 接口** → `/api/`
3. **命令行** → `/cmd/`
4. **测试** → `/pkg/<module>/<name>_test.go`
5. **文档** → `/docs/`
6. **部署** → `/deploy/` (如需要)

## 外部依赖

主要依赖：
- `google.golang.org/grpc` - gRPC 框架
- `github.com/prometheus/client_golang` - Prometheus 指标
- `github.com/hashicorp/raft` - Raft 实现（或自研）
- `github.com/spf13/viper` - 配置管理
- `github.com/spf13/cobra` - CLI 框架
- `github.com/twmb/murmur3` - MurmurHash3
