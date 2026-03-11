# 分布式布隆过滤器 - 项目概览

> **项目状态**: 设计阶段  
> **创建时间**: 2026-03-11  
> **许可证**: Apache 2.0

## 📋 项目信息

| 项目 | 值 |
|------|-----|
| **名称** | Distributed Bloom Filter (DBF) |
| **场景** | 海量数据存在性判存 |
| **数据量** | 10 亿 + |
| **QPS** | 10 万 + |
| **误判率** | 0.1% |
| **语言** | Go 1.21+ |
| **部署** | Kubernetes 原生 |
| **核心特性** | 支持删除、多副本同步、持久化 |

## 🎯 核心需求

- ✅ 海量数据判存（10 亿级）
- ✅ 支持高并发读写（10 万 QPS）
- ✅ 低误判率（0.1%）
- ✅ **支持删除操作**（Counting Bloom Filter）
- ✅ **数据持续增量写入**
- ✅ **持久化**（WAL + 快照）
- ✅ **多节点数据同步**（Raft 共识）
- ✅ Kubernetes 部署

## 🏗️ 架构设计

### 整体架构

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
│            │   PV (WAL+快照)  │  (持久化)                │
│            └─────────────────┘                          │
└─────────────────────────────────────────────────────────┘
```

### 技术栈

| 组件 | 技术选型 |
|------|----------|
| **语言** | Go 1.21+ |
| **通信** | gRPC + Protocol Buffers |
| **共识** | Raft |
| **数据结构** | Counting Bloom Filter (4-bit 计数器) |
| **哈希** | MurmurHash3 + Double Hashing |
| **持久化** | WAL (顺序写) + 压缩快照 |
| **部署** | Kubernetes (StatefulSet + Deployment) |
| **监控** | Prometheus + Grafana |
| **日志** | 结构化日志 (JSON) |

### 关键设计

#### 1. Counting Bloom Filter

```
标准 Bloom Filter → 不支持删除
Counting Bloom Filter → 每个位用 4-bit 计数器 → 支持删除

内存计算:
  10 亿数据 / 6 节点 = 1.67 亿/节点
  计数器数 = 14.4 × 1.67 亿 ≈ 2.4 亿
  内存 = 2.4 亿 × 4 bits ≈ 120 MB/节点
  总内存 ≈ 720 MB (6 节点集群)
```

#### 2. Raft 数据同步

```
写入流程:
  Client → Leader → 追加 WAL → 复制到 Followers → 多数派确认 → 提交 → 返回

读取流程:
  Client → Leader → 内存查询 → 返回

故障恢复:
  Follower 故障 → Leader 检测 → 标记 unhealthy → K8s 重启 → 从 WAL 恢复 + 同步日志
  Leader 故障 → Followers 选举 → 新 Leader → 更新路由 → 恢复服务 (<500ms)
```

#### 3. 一致性 Hash 分片

```
Hash 空间 (2^32) → 分成 N 个分片 → 每个分片 3 副本
扩容时 → 只影响相邻分片 → 后台渐进式迁移
```

## 📁 项目结构

```
distributed-bloom-filter/
├── README.md                 # 项目说明
├── ARCHITECTURE.md           # 架构设计（重点：多节点同步）
├── CONTRIBUTING.md           # 贡献指南
├── CHANGELOG.md              # 变更日志
├── LICENSE                   # Apache 2.0
├── PROJECT_STRUCTURE.md      # 目录结构说明
├── config.example.yaml       # 配置示例
│
├── cmd/                      # 可执行文件入口
├── pkg/                      # 核心库代码
│   ├── bloom/                # Counting Bloom Filter
│   ├── raft/                 # Raft 共识
│   ├── storage/              # 持久化 (WAL+ 快照)
│   ├── cluster/              # 集群管理
│   └── metrics/              # 监控指标
├── api/                      # gRPC API
├── deploy/                   # K8s 部署配置
└── docs/                     # 详细文档
```

## 📄 文档索引

| 文档 | 说明 |
|------|------|
| [README.md](README.md) | 项目说明、快速开始、使用示例 |
| [ARCHITECTURE.md](ARCHITECTURE.md) | **详细架构设计，多节点同步机制** |
| [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) | 项目目录结构说明 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 贡献指南 |
| [deploy/README.md](deploy/README.md) | K8s 部署指南 |
| [config.example.yaml](config.example.yaml) | 配置示例 |

## 🚀 下一步计划

### 阶段 1: 核心实现 (2-3 周)
- [ ] Counting Bloom Filter 数据结构
- [ ] Double Hashing 实现
- [ ] 单元测试 + 基准测试

### 阶段 2: 分布式层 (3-4 周)
- [ ] Raft 共识实现（或集成 HashiCorp Raft）
- [ ] WAL 持久化
- [ ] 快照管理
- [ ] 分片路由

### 阶段 3: API 与服务 (2 周)
- [ ] gRPC 服务实现
- [ ] API Gateway
- [ ] Go SDK

### 阶段 4: 部署与监控 (2 周)
- [ ] K8s 部署配置
- [ ] Prometheus 指标
- [ ] Grafana 仪表盘
- [ ] 日志系统

### 阶段 5: 测试与优化 (2-3 周)
- [ ] 性能测试（验证 10 万 QPS）
- [ ] 故障注入测试
- [ ] 优化调优
- [ ] 文档完善

### 阶段 6: 发布准备 (1 周)
- [ ] 代码审查
- [ ] 安全审计
- [ ] 发布 v0.1.0
- [ ] 开源公告

## 📊 性能目标

| 指标 | 目标值 | 验证方法 |
|------|--------|----------|
| QPS | 10 万 + | 压测 (wrk/grpcurl) |
| P99 延迟 | < 5ms | Prometheus 指标 |
| 误判率 | < 0.1% | 理论计算 + 实测 |
| 故障恢复时间 | < 500ms | 故障注入测试 |
| 启动恢复时间 | < 30s | 重启测试 |

## 🔒 安全考虑

- [ ] TLS 加密通信
- [ ] 认证授权（可选）
- [ ] 限流防刷
- [ ] 敏感配置（密钥）不提交
- [ ] 依赖漏洞扫描

## 📝 待确认事项

- [ ] 项目组织名（`yourorg` 占位符）
- [ ] GitHub 仓库名
- [ ] Docker Hub 组织名
- [ ] 是否使用现有 Raft 库（推荐 HashiCorp Raft）
- [ ] CI/CD 平台选择（GitHub Actions / GitLab CI）

## 🎉 开源发布清单

- [x] README.md
- [x] ARCHITECTURE.md
- [x] LICENSE
- [x] CONTRIBUTING.md
- [x] CHANGELOG.md
- [x] .gitignore
- [x] 配置示例
- [x] 部署文档
- [ ] 核心代码实现
- [ ] 单元测试
- [ ] CI/CD 配置
- [ ] Docker 镜像
- [ ] Go Module 发布
- [ ] 发布公告

---

**项目负责人**: @wm  
**技术负责人**: [待指定]  
**预计发布时间**: 2026-Q2

---

*Last updated: 2026-03-11*
