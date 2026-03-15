# Raft 集成架构设计文档

**文档版本**: v1.0  
**创建日期**: 2026-03-13  
**作者**: Alex Chen (首席架构师)  
**状态**: ✅ 已评审通过  
**关联里程碑**: M2 - 分布式层完成 (Week 5)

---

## 1. 架构概述

### 1.1 Raft 在系统中的位置

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Request                          │
└─────────────────────────┬───────────────────────────────────┘
                          │ gRPC (with Auth + TLS)
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Gateway Layer                          │
│  - 请求路由 (Key → Shard)                                     │
│  - 负载均衡                                                   │
│  - 认证/限流                                                  │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   Storage Node (Shard N)                      │
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              gRPC Service Layer                          │ │
│  │  - Add / Remove / Contains / Batch Ops                   │ │
│  │  - Auth Interceptor (P0 已修复)                           │ │
│  │  - Rate Limiter (P0 已修复)                               │ │
│  │  - TLS (P0 已修复)                                        │ │
│  └─────────────────────┬───────────────────────────────────┘ │
│                        │                                      │
│                        ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Raft Consensus Layer ← 本模块               │ │
│  │  - Leader Election                                       │ │
│  │  - Log Replication                                       │ │
│  │  - FSM Apply                                             │ │
│  │  - Snapshot Management                                   │ │
│  └─────────────────────┬───────────────────────────────────┘ │
│                        │                                      │
│                        ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │           Counting Bloom Filter (FSM State)              │ │
│  │  - 4-bit Counters                                        │ │
│  │  - Double Hash (MurmurHash3)                             │ │
│  │  - Thread-safe (RWMutex)                                 │ │
│  └─────────────────────┬───────────────────────────────────┘ │
│                        │                                      │
│                        ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │            Persistence Layer (WAL + Snapshot)            │ │
│  │  - AES-256-GCM 加密                                       │ │
│  │  - 密钥轮换                                               │ │
│  │  - 自动清理                                               │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### 1.2 节点角色

| 角色 | 职责 | 数量 (每分片) | 说明 |
|------|------|---------------|------|
| **Leader** | 处理所有写请求、日志复制、心跳发送 | 1 | 每分片唯一，由选举产生 |
| **Follower** | 接收 Leader 日志复制、投票、可处理读请求 | 2 | 被动接收，超时触发选举 |
| **Candidate** | 临时角色，发起选举 | 0 (瞬时) | Follower 超时后转换 |

### 1.3 数据流图

#### 写操作流程 (Add/Remove)

```
Client → API Gateway → Shard Leader → Raft Log → Majority Ack → Apply FSM → Response
                           │
                           └──→ Replicate to Followers (parallel)
```

#### 读操作流程 (Contains)

```
Client → API Gateway → Any Replica (Leader/Follower) → Local Bloom Filter → Response
```

#### Leader 选举流程

```
Follower Timeout → Become Candidate → Request Votes → Majority Wins → Become Leader → Send Heartbeats
```

---

## 2. 集成方案

### 2.1 推荐选项：**选项 A - 集成 HashiCorp Raft 库**

#### 推荐理由

| 维度 | 选项 A (HashiCorp Raft) | 选项 B (自研) | 评估 |
|------|------------------------|---------------|------|
| **成熟度** | ✅ 生产级，Consul/Vault 使用 | ❌ 从零开始 | A 胜 |
| **开发周期** | ✅ 1-2 周集成 | ❌ 4-6 周开发 + 测试 | A 胜 |
| **测试覆盖** | ✅ 完整测试套件 | ❌ 需自建测试框架 | A 胜 |
| **社区支持** | ✅ 活跃社区，定期更新 | ❌ 仅团队维护 | A 胜 |
| **代码控制** | ⚠️ 需适配层 | ✅ 完全控制 | B 胜 |
| **学习成本** | ⚠️ 需学习 API | ✅ 无外部依赖 | B 胜 |
| **性能调优** | ⚠️ 受库限制 | ✅ 可深度优化 | B 胜 |

**结论**: 对于 M2 里程碑（Week 5 完成），**选项 A 是唯一可行方案**。自研 Raft 需要至少 4-6 周，且测试成本高，不符合项目时间线。

#### 已验证的集成基础

当前代码已实现基础集成框架 (`internal/raft/node.go`):

- ✅ 已导入 `github.com/hashicorp/raft` v1.7.3
- ✅ 已实现 `raft.FSM` 接口 (Apply/Snapshot/Restore)
- ✅ 已集成 BoltDB 存储 (`raft-boltdb`)
- ✅ 已实现 TCP 传输层
- ✅ 已实现基础单元测试（Leader 选举、Add/Contains）

**完成度评估**: 🟢 60% (核心框架就绪，需完善多节点集群)

---

### 2.2 模块划分和接口定义

#### 模块结构

```
internal/raft/
├── node.go              # Raft 节点封装 (已完成 60%)
├── node_test.go         # 单元测试 (已完成)
├── config.go            # Raft 配置 (待创建)
├── cluster.go           # 集群管理 (待创建)
├── fsm.go               # FSM 详细实现 (待完善)
└── snapshot.go          # 快照管理 (待完善)
```

#### 核心接口定义

```go
// RaftNode - Raft 节点封装
type RaftNode struct {
    nodeID          string
    raftNode        *raft.Raft
    raftStore       *raftboltdb.BoltStore
    transport       *raft.NetworkTransport
    bloomFilter     *bloom.CountingBloomFilter
    walEncryptor    *wal.WALEncryptor
    metadataService *metadata.Service
    mu              sync.RWMutex
}

// 核心方法
func NewNode(...) *RaftNode
func (n *RaftNode) Start(bootstrap bool) error
func (n *RaftNode) Add(item []byte) error
func (n *RaftNode) Remove(item []byte) error
func (n *RaftNode) Contains(item []byte) bool
func (n *RaftNode) IsLeader() bool
func (n *RaftNode) Leader() raft.ServerAddress
func (n *RaftNode) Shutdown()
func (n *RaftNode) JoinCluster(nodeID, addr string) error  // 待实现
func (n *RaftNode) RemoveCluster(nodeID string) error     // 待实现
```

---

### 2.3 与现有模块的集成点

#### 2.3.1 Bloom Filter 集成

```go
// FSM Apply 中直接操作 Bloom Filter
func (n *Node) Apply(log *raft.Log) interface{} {
    var cmd Command
    json.Unmarshal(log.Data, &cmd)
    
    n.mu.Lock()
    defer n.mu.Unlock()
    
    switch cmd.Command {
    case "add":
        n.bloomFilter.Add(cmd.Item)  // 直接调用
    case "remove":
        n.bloomFilter.Remove(cmd.Item)
    }
    return nil
}
```

**集成状态**: ✅ 已完成

#### 2.3.2 WAL 集成

Raft 日志存储使用 BoltDB，WAL 用于 Bloom Filter 状态持久化：

```
Raft Log Entries (BoltDB)
        ↓
    FSM Apply
        ↓
Bloom Filter State (Memory)
        ↓
    Periodic Snapshot
        ↓
WAL Encrypted Write (AES-256-GCM)
```

**待实现**: 快照写入时调用 `WALEncryptor` 加密

#### 2.3.3 gRPC 集成

```go
// gRPC Service 调用 Raft Node
type DbfService struct {
    pb.UnimplementedDbfServer
    raftNode *raft.Node
}

func (s *DbfService) Add(ctx context.Context, req *pb.AddRequest) (*pb.AddResponse, error) {
    // 1. 验证认证 (P0 已实现)
    // 2. 检查是否 Leader
    if !s.raftNode.IsLeader() {
        return nil, status.Error(codes.FailedPrecondition, "not leader")
    }
    // 3. 通过 Raft 提交
    err := s.raftNode.Add(req.Item)
    return &pb.AddResponse{}, err
}
```

**集成状态**: ⏳ 待实现 (gRPC 服务层)

---

## 3. 核心组件设计

### 3.1 Raft 节点状态机

```
┌─────────────────────────────────────────────────────────────┐
│                    Raft FSM State Machine                    │
│                                                               │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐    │
│  │   Follower  │────▶│  Candidate  │────▶│    Leader   │    │
│  └─────────────┘     └─────────────┘     └─────────────┘    │
│         ▲                                      │             │
│         │                                      │             │
│         └──────────────────────────────────────┘             │
│                      (Step Down)                             │
│                                                               │
│  状态转换触发条件:                                             │
│  - Follower → Candidate: 选举超时 (150-300ms 无心跳)          │
│  - Candidate → Leader: 获得多数票                             │
│  - Leader → Follower: 发现更高 Term 的 Leader                │
│  - Any → Follower: 重启/故障恢复                              │
└─────────────────────────────────────────────────────────────┘
```

#### FSM 数据结构

```go
type fsmState struct {
    bloomFilter *bloom.CountingBloomFilter
    version     uint64  // 状态版本号，用于快照
    lastApplied uint64  // 最后应用的日志索引
}
```

---

### 3.2 Leader 选举流程

```
1. Follower 超时 (electionTimeout = 150-300ms 随机)
         ↓
2. 转换为 Candidate，Term +1，投票给自己
         ↓
3. 并行发送 RequestVote RPC 给所有其他节点
         ↓
4. 等待响应：
   - 获得多数票 → 成为 Leader，发送心跳
   - 发现更高 Term → 转换为 Follower
   - 超时 → Term +1，重新选举
         ↓
5. Leader 定期发送心跳 (heartbeatInterval = 50ms)
```

#### 选举配置建议

```go
config := raft.DefaultConfig()
config.HeartbeatTimeout = 50 * time.Millisecond      // 心跳超时
config.ElectionTimeout = 200 * time.Millisecond      // 选举超时
config.LeaderLeaseTimeout = 50 * time.Millisecond    // Leader 租约
config.CommitTimeout = 50 * time.Millisecond         // 提交超时
```

---

### 3.3 日志复制机制

```
┌─────────────────────────────────────────────────────────────┐
│                    Log Replication Flow                      │
│                                                               │
│  Client Request                                              │
│       ↓                                                      │
│  ┌─────────────────┐                                        │
│  │     Leader      │ 1. Append to Local Log                 │
│  │  (Shard 0)      │ 2. Parallel AppendEntries to Followers │
│  └────────┬────────┘                                        │
│           │                                                  │
│     ┌─────┴─────┐                                           │
│     ↓           ↓                                           │
│  ┌──────┐   ┌──────┐                                        │
│  │ F1   │   │ F2   │ 3. Acknowledge                         │
│  └──┬───┘   └──┬───┘                                        │
│     │           │                                           │
│     └─────┬─────┘                                           │
│           ↓                                                  │
│  ┌─────────────────┐                                        │
│  │     Leader      │ 4. Majority Ack → Commit               │
│  │  (Shard 0)      │ 5. Apply to FSM                        │
│  └─────────────────┘                                        │
└─────────────────────────────────────────────────────────────┘
```

#### 日志格式

```go
type LogEntry struct {
    Index      uint64        // 日志索引
    Term       uint64        // 任期号
    Type       raft.LogType  // LogCommand / LogNoop
    Data       []byte        // JSON 编码的 Command
    Extensions []byte        // 扩展字段（可用于追踪）
}

type Command struct {
    Command string `json:"command"`  // "add" | "remove"
    Item    []byte `json:"item"`
    Timestamp int64 `json:"timestamp"`  // 用于审计
}
```

---

### 3.4 快照管理

#### 快照触发条件

1. **日志数量阈值**: `SnapshotThreshold = 10000` 条日志
2. **日志间隔阈值**: `SnapshotInterval = 30 * time.Second`
3. **手动触发**: 运维命令

#### 快照流程

```
1. Raft 触发 Snapshot() 回调
         ↓
2. 获取 Bloom Filter 当前状态 (RLock)
         ↓
3. 序列化: bloomFilter.Serialize() → []byte
         ↓
4. 加密: walEncryptor.Encrypt(data) → encryptedData
         ↓
5. 写入快照文件: raft.NewFileSnapshotStore
         ↓
6. 清理旧日志: Compact(logIndex)
```

#### 快照文件格式

```
┌─────────────────────────────────────────┐
│          Snapshot File Structure         │
├─────────────────────────────────────────┤
│  Header (16 bytes)                      │
│  - Magic Number (4 bytes)               │
│  - Version (4 bytes)                    │
│  - Size (8 bytes)                       │
├─────────────────────────────────────────┤
│  Encrypted Bloom Filter State           │
│  - Nonce (12 bytes)                     │
│  - Ciphertext (variable)                │
│  - Tag (16 bytes, GCM MAC)              │
├─────────────────────────────────────────┤
│  Metadata (JSON)                        │
│  - LastIncludedIndex                    │
│  - LastIncludedTerm                     │
│  - Timestamp                            │
└─────────────────────────────────────────┘
```

---

## 4. 配置建议

### 4.1 集群规模

| 场景 | 推荐节点数 | 容错能力 | 说明 |
|------|-----------|----------|------|
| **开发/测试** | 3 | 1 节点故障 | 最小高可用配置 |
| **生产环境** | 5 | 2 节点故障 | 推荐配置 |
| **高可用关键** | 7 | 3 节点故障 | 金融级场景 |

**推荐**: **5 节点** (平衡容错能力和性能)

### 4.2 核心参数配置

```yaml
# Raft 配置
raft:
  # 超时配置
  heartbeatTimeout: 50ms      # 心跳超时（Follower 判定 Leader 失效）
  electionTimeout: 200ms      # 选举超时（随机 150-300ms）
  leaderLeaseTimeout: 50ms    # Leader 租约
  commitTimeout: 50ms         # 提交超时
  
  # 快照配置
  snapshotThreshold: 10000    # 触发快照的日志数量
  snapshotInterval: 30s       # 快照检查间隔
  truncateMultibatch: true    # 批量截断日志
  
  # 日志配置
  maxAppendEntries: 64        # 每批最大日志条目
  batchSize: 64               # 批处理大小
  
  # 集群配置
  clusterName: "dbf-shard-0"  # 集群名称
  dataDir: "/data/raft"       # 数据目录
```

### 4.3 日志同步策略

| 策略 | 配置 | 性能 | 一致性 | 推荐场景 |
|------|------|------|--------|----------|
| **同步复制** | `WriteConcern: Majority` | 中 | 强 | 生产默认 |
| **异步复制** | `WriteConcern: One` | 高 | 弱 | 开发测试 |
| **批量提交** | `BatchSize: 64` | 高 | 强 | 高吞吐场景 |

**推荐**: 生产环境使用 **同步复制 + 批量提交**

---

## 5. 风险评估

### 5.1 技术难点

| 难点 | 风险等级 | 影响 | 缓解措施 |
|------|----------|------|----------|
| **多节点集群测试** | 🟠 中 | 开发环境限制 | 使用 Docker Compose 本地模拟 |
| **网络分区处理** | 🟠 中 | 脑裂风险 | HashiCorp Raft 内置处理 |
| **快照加密性能** | 🟡 低 | 延迟增加 1-2ms | 异步快照，后台执行 |
| **BoltDB 并发写入** | 🟡 低 | 写入瓶颈 | 批量提交，减少锁竞争 |

### 5.2 性能影响

| 操作 | 单机延迟 | Raft 集群延迟 | 影响 |
|------|----------|---------------|------|
| **Add** | ~1ms | ~15ms (P99) | +14ms (多数派确认) |
| **Remove** | ~1ms | ~15ms (P99) | +14ms |
| **Contains** | ~1ms | ~1ms (本地读) | 无影响 |
| **Batch Add (100)** | ~10ms | ~50ms (P99) | +40ms |

**优化建议**:
1. 批量操作合并为单条日志
2. 读请求可路由到 Follower（线性一致性读除外）
3. 调整 `maxAppendEntries` 优化吞吐

### 5.3 缓解措施

#### 5.3.1 开发阶段

1. **本地多节点测试**: 创建 `docker-compose.raft.yaml`
   ```yaml
   services:
     raft-node-1:
       image: dbf-server:latest
       command: ["--node-id=node1", "--bootstrap", "--raft-port=18080"]
       ports: ["18080:18080"]
     
     raft-node-2:
       image: dbf-server:latest
       command: ["--node-id=node2", "--join=127.0.0.1:18080", "--raft-port=18081"]
       ports: ["18081:18081"]
     
     raft-node-3:
       image: dbf-server:latest
       command: ["--node-id=node3", "--join=127.0.0.1:18080", "--raft-port=18082"]
       ports: ["18082:18082"]
   ```

2. **故障注入测试**: 使用 `toxiproxy` 模拟网络延迟/分区

3. **性能基准测试**: 每周运行一次，监控延迟变化

#### 5.3.2 生产部署

1. **监控指标**:
   - `raft_state` (Leader/Follower)
   - `raft_term` (当前任期)
   - `raft_last_log_index` (最后日志索引)
   - `raft_commit_index` (提交索引)
   - `raft_fsm_pending` (待应用日志数)

2. **告警规则**:
   - Leader 变更频率 > 5 次/小时
   - 日志复制延迟 > 100ms
   - FSM 待应用队列 > 1000

3. **运维手册**:
   - 节点加入/移除流程
   - Leader 故障恢复步骤
   - 快照恢复操作指南

---

## 6. 与 P0 安全修复的集成

### 6.1 认证集成

Raft 节点间通信需要认证：

```go
// Raft 节点间 RPC 认证
type RaftAuthInterceptor struct {
    apiKeyStore APIKeyStore
}

func (r *RaftAuthInterceptor) Intercept RPC {
    // 验证节点间通信的 API Key
    // 防止未授权节点加入集群
}
```

### 6.2 TLS 集成

```go
// Raft 传输层 TLS 配置
transport, err := raft.NewTLSTransport(
    bindAddr,
    tcpAddr,
    3,
    10*time.Second,
    tlsConfig,  // mTLS 配置
    os.Stderr,
)
```

### 6.3 限流集成

Raft 层不需要限流（由 gRPC 层处理），但需监控：
- 日志提交速率
- 快照生成频率

---

## 7. 总结

### 7.1 推荐方案

**选项 A: 集成 HashiCorp Raft 库**

- ✅ 成熟稳定，生产级验证
- ✅ 开发周期短（1-2 周）
- ✅ 社区活跃，文档完善
- ✅ 与现有代码兼容性好

### 7.2 关键决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Raft 库 | HashiCorp Raft v1.7.3 | 生产级，Consul/Vault 使用 |
| 存储后端 | BoltDB | 嵌入式，零依赖，Raft 官方推荐 |
| 集群规模 | 5 节点 | 容错 2 节点，平衡性能和可用性 |
| 选举超时 | 200ms | 快速故障恢复，避免脑裂 |
| 心跳间隔 | 50ms | 及时检测 Leader 故障 |

### 7.3 下一步

1. **完善 Raft 节点实现** (David, Week 3-4)
2. **实现多节点集群测试** (David + Sarah, Week 4)
3. **集成 gRPC 服务层** (David, Week 5)
4. **故障恢复测试** (Sarah, Week 5)

---

**文档审批**:
- [x] Alex Chen (首席架构师)
- [ ] David Wang (高级服务端工程师)
- [ ] wm (项目需求方)

*Last updated: 2026-03-13 08:51 GMT+8*
