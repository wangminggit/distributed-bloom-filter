# 架构设计文档

## 系统架构

### 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                        │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                     Client Layer                          │  │
│   │  ┌─────────┐ ┌─────────┐ ┌─────────┐                     │  │
│   │  │ Go SDK  │ │ gRPC    │ │ HTTP    │                     │  │
│   │  │         │ │ Client  │ │ Client  │                     │  │
│   │  └────┬────┘ └────┬────┘ └────┬────┘                     │  │
│   └───────┼───────────┼───────────┼───────────────────────────┘  │
│           │           │           │                               │
│           └───────────┼───────────┘                               │
│                       ▼                                           │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                    API Gateway Layer                      │  │
│   │  ┌─────────────────────────────────────────────────────┐  │  │
│   │  │              Load Balancer (gRPC)                    │  │  │
│   │  │         (K8s Service + Envoy/NGINX)                  │  │  │
│   │  └─────────────────────┬───────────────────────────────┘  │  │
│   │                        │                                   │  │
│   │  ┌─────────────────────▼───────────────────────────────┐  │  │
│   │  │              API Gateway (Stateless)                 │  │  │
│   │  │  - 请求路由 (基于一致性 Hash)                           │  │  │
│   │  │  - 负载均衡                                           │  │  │
│   │  │  - 限流/认证                                          │  │  │
│   │  │  - 副本数：2-3 (HPA 自动扩缩容)                        │  │  │
│   │  └─────────────────────┬───────────────────────────────┘  │  │
│   └────────────────────────┼──────────────────────────────────┘  │
│                            │                                     │
│         ┌──────────────────┼──────────────────┐                  │
│         │                  │                  │                  │
│         ▼                  ▼                  ▼                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                   Storage Layer (StatefulSet)             │  │
│   │                                                           │  │
│   │  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐ │  │
│   │  │   Shard 0   │     │   Shard 1   │     │   Shard 2   │ │  │
│   │  │  ┌───────┐  │     │  ┌───────┐  │     │  ┌───────┐  │ │  │
│   │  │  │Leader │  │────▶│  │Leader │  │────▶│  │Leader │  │ │  │
│   │  │  │ F:1,2 │  │     │  │ F:0,2 │  │     │  │ F:0,1 │  │ │  │
│   │  │  └───┬───┘  │     │  └───┬───┘  │     │  └───┬───┘  │ │  │
│   │  │      │      │     │      │      │     │      │      │ │  │
│   │  │  ┌───▼───┐  │     │  ┌───▼───┐  │     │  ┌───▼───┐  │ │  │
│   │  │  │Follower│  │     │  │Follower│  │     │  │Follower│  │ │  │
│   │  │  │  (1)  │  │     │  │  (0)  │  │     │  │  (2)  │  │ │  │
│   │  │  └───────┘  │     │  └───────┘  │     │  └───────┘  │ │  │
│   │  │  ┌───────┐  │     │  ┌───────┐  │     │  ┌───────┐  │ │  │
│   │  │  │Follower│  │     │  │Follower│  │     │  │Follower│  │ │  │
│   │  │  │  (2)  │  │     │  │  (2)  │  │     │  │  (1)  │  │ │  │
│   │  │  └───────┘  │     │  └───────┘  │     │  └───────┘  │ │  │
│   │  └─────────────┘     └─────────────┘     └─────────────┘ │  │
│   │                                                           │  │
│   │  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐ │  │
│   │  │   Shard 3   │     │   Shard 4   │     │   Shard 5   │ │  │
│   │  │  ┌───────┐  │     │  ┌───────┐  │     │  ┌───────┐  │ │  │
│   │  │  │Leader │  │────▶│  │Leader │  │────▶│  │Leader │  │ │  │
│   │  │  │ F:4,5 │  │     │  │ F:3,5 │  │     │  │ F:3,4 │  │ │  │
│   │  │  └───┬───┘  │     │  └───┬───┘  │     │  └───┬───┘  │ │  │
│   │  │      │      │     │      │      │     │      │      │ │  │
│   │  │  ┌───▼───┐  │     │  ┌───▼───┐  │     │  ┌───▼───┐  │ │  │
│   │  │  │Follower│  │     │  │Follower│  │     │  │Follower│  │ │  │
│   │  │  │  (4)  │  │     │  │  (3)  │  │     │  │  (3)  │  │ │  │
│   │  │  └───────┘  │     │  └───────┘  │     │  └───────┘  │ │  │
│   │  │  ┌───────┐  │     │  ┌───────┐  │     │  ┌───────┐  │ │  │
│   │  │  │Follower│  │     │  │Follower│  │     │  │Follower│  │ │  │
│   │  │  │  (5)  │  │     │  │  (5)  │  │     │  │  (4)  │  │ │  │
│   │  │  └───────┘  │     │  └───────┘  │     │  └───────┘  │ │  │
│   │  └─────────────┘     └─────────────┘     └─────────────┘ │  │
│   │                                                           │  │
│   │  每个分片 = 1 Leader + 2 Followers (3 副本)                  │  │
│   │  总计：18 个物理 Pod，6 个逻辑分片                           │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                    Metadata Layer                         │  │
│   │  ┌─────────────────────────────────────────────────────┐ │  │
│   │  │         K8s ConfigMap + Gossip Protocol              │ │  │
│   │  │  - 集群元数据 (ConfigMap)                            │ │  │
│   │  │  - 节点发现与健康状态 (Gossip)                        │ │  │
│   │  │  - 分片映射表 (ConfigMap + 本地缓存)                   │ │  │
│   │  └─────────────────────────────────────────────────────┘ │  │
│   └──────────────────────────────────────────────────────────┘  │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                    Persistence Layer                      │  │
│   │  ┌─────────────────────────────────────────────────────┐ │  │
│   │  │         Persistent Volumes (WAL+ 快照，加密)           │ │  │
│   │  │  - AES-256 加密                                       │ │  │
│   │  │  - 每分片独立 PV                                      │ │  │
│   │  └─────────────────────────────────────────────────────┘ │  │
│   └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. API Gateway (无状态服务)

**职责**:
- 接收客户端 gRPC/HTTP 请求
- 根据 key 的 hash 值路由到对应分片的 Leader
- 负载均衡、限流、认证
- 请求聚合（批量操作）

**部署**:
- Kubernetes Deployment
- 副本数：2-3（HPA 自动扩缩容）
- 资源：512Mi 内存，500m CPU

**路由算法**:
```go
func routeToShard(key string, shardCount int) int {
    hash := murmur3.Sum32([]byte(key))
    return int(hash) % shardCount
}
```

### 2. Storage Node (有状态服务)

**职责**:
- 存储 Counting Bloom Filter 数据
- 处理 Add/Delete/Contains 操作
- WAL 写入 + 定期快照（AES-256 加密）
- 参与 HashiCorp Raft 共识，数据同步

**部署**:
- Kubernetes StatefulSet
- 副本数：18（6 分片 × 3 副本）
- 资源：1Gi 内存，1 CPU（每分片）
- 持久化：10Gi PV（WAL + 快照，加密）

**内存分配**（6 分片场景）:
```
总内存需求：6 分片 × 10 亿元素 × 4 bit/计数器 × 2 (double hash) / 8 = 6GB
每分片内存：1GB (计数器数组)
每 Pod 内存：1GB (单分片副本) + 512MB (Raft + WAL 缓冲) = 1.5Gi
实际配置：2Gi (预留 33% 缓冲)
```

**QPS 分配**（6 分片场景）:
```
假设总 QPS 目标：60,000 QPS
每分片 QPS: 60,000 / 6 = 10,000 QPS
每 Leader QPS: 10,000 QPS (处理写入)
每 Follower QPS: 10,000 QPS (日志复制 + 可选读)
单 Pod 总 QPS: 20,000 QPS (Leader + Follower 角色)

写入延迟预算:
- Leader 处理：5ms
- 多数派确认 (2/3): 10ms
- 总延迟：15ms (P99)
```

**内部结构**:
```
┌─────────────────────────────────────┐
│           Storage Node               │
│                                      │
│  ┌─────────────────────────────────┐│
│  │   HashiCorp Raft Consensus       ││
│  │  - Leader Election               ││
│  │  - Log Replication               ││
│  │  - FSM Apply                     ││
│  │  - Snapshot Management           ││
│  └────────────────┬────────────────┘│
│                   │                  │
│  ┌────────────────▼────────────────┐│
│  │      Counting Bloom Filter       ││
│  │  - Counter Array (4-bit)         ││
│  │  - Hash Functions (twmb/murmur3) ││
│  │  - Element Count                 ││
│  └────────────────┬────────────────┘│
│                   │                  │
│  ┌────────────────▼────────────────┐│
│  │      Persistence Engine          ││
│  │  - WAL (AES-256 加密)             ││
│  │  - Snapshot (Compressed + 加密)   ││
│  │  - Recovery                      ││
│  └─────────────────────────────────┘│
└─────────────────────────────────────┘
```

### 3. 元数据服务 (K8s ConfigMap + Gossip)

**架构设计**:

不再依赖 etcd，采用轻量级方案：

```
┌─────────────────────────────────────────────────────────────┐
│                    Metadata Architecture                     │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                  K8s ConfigMap                          │ │
│  │  Name: dbf-cluster-metadata                             │ │
│  │                                                         │ │
│  │  Data:                                                  │ │
│  │  - cluster.json: 集群配置 (version, shardCount, etc.)   │ │
│  │  - shard-map.json: 分片映射表                           │ │
│  │  - nodes.json: 节点注册信息                             │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│                          │ Watch/Update                      │
│                          ▼                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              Each Storage Node                          │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │  Local Metadata Cache (in-memory)                │  │ │
│  │  │  - 缓存 ConfigMap 内容                             │  │ │
│  │  │  - 通过 K8s Watch API 实时更新                     │  │ │
│  │  │  - 离线时可使用本地缓存                            │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  │                         │                                │ │
│  │                         │ Gossip Protocol                │ │
│  │                         ▼                                │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │  Memberlist (HashiCorp)                          │  │ │
│  │  │  - 节点发现 (Node Discovery)                      │  │ │
│  │  │  - 健康检查 (Heartbeat)                           │  │ │
│  │  │  - 故障检测 (Failure Detection)                   │  │ │
│  │  │  - 成员变更广播 (Membership Change)               │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**ConfigMap 存储内容**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dbf-cluster-metadata
  namespace: dbf-system
data:
  cluster.json: |
    {
      "version": "1.0.0",
      "shardCount": 6,
      "replicationFactor": 3,
      "createdAt": "2026-03-11T00:00:00Z"
    }
  
  shard-map.json: |
    {
      "shards": [
        {
          "id": 0,
          "range": [0, 715827882],
          "leader": "dbf-storage-0",
          "followers": ["dbf-storage-1", "dbf-storage-2"],
          "hashRange": "0x00000000-0x2A000000"
        },
        {
          "id": 1,
          "range": [715827883, 1431655765],
          "leader": "dbf-storage-3",
          "followers": ["dbf-storage-4", "dbf-storage-5"],
          "hashRange": "0x2A000001-0x55000000"
        },
        {
          "id": 2,
          "range": [1431655766, 2147483647],
          "leader": "dbf-storage-6",
          "followers": ["dbf-storage-7", "dbf-storage-8"],
          "hashRange": "0x55000001-0x7FFFFFFF"
        },
        {
          "id": 3,
          "range": [2147483648, 2863311530],
          "leader": "dbf-storage-9",
          "followers": ["dbf-storage-10", "dbf-storage-11"],
          "hashRange": "0x80000000-0xAAAAAAAA"
        },
        {
          "id": 4,
          "range": [2863311531, 3579139413],
          "leader": "dbf-storage-12",
          "followers": ["dbf-storage-13", "dbf-storage-14"],
          "hashRange": "0xAAAAAAAA+1-0xD5000000"
        },
        {
          "id": 5,
          "range": [3579139414, 4294967295],
          "leader": "dbf-storage-15",
          "followers": ["dbf-storage-16", "dbf-storage-17"],
          "hashRange": "0xD5000001-0xFFFFFFFF"
        }
      ]
    }
  
  nodes.json: |
    {
      "nodes": [
        {
          "name": "dbf-storage-0",
          "addr": "10.0.0.1:50052",
          "shard": 0,
          "role": "leader",
          "status": "healthy",
          "lastHeartbeat": 1710144000
        },
        {
          "name": "dbf-storage-1",
          "addr": "10.0.0.2:50052",
          "shard": 0,
          "role": "follower",
          "status": "healthy",
          "lastHeartbeat": 1710144000
        }
        // ... 其他节点
      ]
    }
```

**Gossip 协议实现**（使用 HashiCorp Memberlist）:

```go
// 节点启动时
func (n *Node) startGossip() error {
    // 1. 配置 Memberlist
    config := memberlist.DefaultLANConfig()
    config.Name = n.nodeName
    config.BindAddr = n.bindAddr
    config.BindPort = n.bindPort
    config.Delegate = n  // 实现 Delegate 接口
    
    // 2. 设置事件代理
    config.Events = &GossipEventHandler{
        OnJoin:  n.handleNodeJoin,
        OnLeave: n.handleNodeLeave,
    }
    
    // 3. 启动 Memberlist
    mlist, err := memberlist.Create(config)
    if err != nil {
        return err
    }
    n.memberlist = mlist
    
    // 4. 加入集群 (种子节点)
    _, err = mlist.Join(n.seedNodes)
    return err
}

// 实现 Delegate 接口
func (n *Node) NodeMeta(limit int) []byte {
    // 返回节点元数据（分片 ID、角色等）
    meta := NodeMeta{
        ShardID: n.shardID,
        Role:    n.role,
        Addr:    n.addr,
    }
    return json.Marshal(meta)
}

func (n *Node) NotifyMsg(buf []byte) {
    // 接收 Gossip 消息
    msg := decodeMessage(buf)
    switch msg.Type {
    case MsgTypeLeaderChange:
        n.handleLeaderChange(msg)
    case MsgTypeHealthCheck:
        n.updateNodeHealth(msg)
    }
}

func (n *Node) GetBroadcasts(overhead, limit int) [][]byte {
    // 生成待广播的消息（状态变更、Leader 选举结果等）
    return n.broadcastQueue.GetBroadcasts(overhead, limit)
}
```

**ConfigMap 更新流程**:

```
1. 节点启动:
   - 从 ConfigMap 加载集群元数据
   - 通过 Gossip 加入集群
   - 注册自身信息到 ConfigMap (通过 K8s API)

2. Leader 选举完成:
   - 新 Leader 更新 ConfigMap 中的 shard-map.json
   - 其他节点通过 Watch API 感知变更
   - 更新本地缓存

3. 节点故障:
   - Gossip 检测到节点失联
   - 触发 Leader 选举
   - 更新 ConfigMap 中的节点状态

4. 节点恢复:
   - 从 ConfigMap 获取最新集群状态
   - 通过 Gossip 重新加入
   - 从 Leader 同步缺失数据
```

**优势**:
- ✅ 无需额外部署 etcd，简化架构
- ✅ 利用 K8s 原生能力（ConfigMap + Watch API）
- ✅ Gossip 提供去中心化的节点发现和健康检查
- ✅ 本地缓存保证离线可用性
- ✅ 降低外部依赖，提高系统稳定性

---

## 多节点数据同步机制

### 同步方案选择

我们采用 **HashiCorp Raft** 实现多节点数据同步：

| 方案 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| HashiCorp Raft | 成熟稳定、Go 语言实现、活跃维护、与 Memberlist 集成 | 写入需多数派确认，延迟略高 | ✅ |
| 自研 Raft | 完全可控、可定制 | 开发成本高、稳定性风险 | ❌ |
| Gossip | 最终一致性、高可用、低延迟 | 可能短暂不一致、实现复杂 | ❌ |
| Primary-Backup | 简单、低延迟 | 单点故障风险 | ❌ |

### HashiCorp Raft 集成设计

#### 架构集成

```
┌─────────────────────────────────────────────────────────────┐
│                    Storage Node Architecture                 │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                 Application Layer                       │ │
│  │  - gRPC/HTTP Handler                                    │ │
│  │  - Add/Delete/Contains Logic                            │ │
│  │  - Counting Bloom Filter (in-memory)                    │ │
│  └─────────────────────┬──────────────────────────────────┘ │
│                        │ FSM Apply                          │
│                        ▼                                    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              HashiCorp Raft Library                     │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │  Raft Core                                        │  │ │
│  │  │  - Leader Election                                │  │ │
│  │  │  - Log Replication                                │  │ │
│  │  │  - Commitment                                     │  │ │
│  │  │  - Snapshot Management                            │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │  Raft Transport                                   │  │ │
│  │  │  - NetworkTransport (TCP)                         │  │ │
│  │  │  - RPC Communication                              │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │  Raft Log Store                                   │  │ │
│  │  │  - BoltDB (embedded)                              │  │ │
│  │  │  - WAL Storage                                    │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  └─────────────────────┬──────────────────────────────────┘ │
│                        │ FSM Interface                      │
│                        ▼                                    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              FSM (Finite State Machine)                 │ │
│  │  - Apply(log *raft.Log) []byte                         │ │
│  │  - Snapshot() (raft.FSMSnapshot, error)                │ │
│  │  - Restore(rc io.ReadCloser) error                     │ │
│  │  - 内部维护 Counting Bloom Filter 状态                   │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

#### 集成代码示例

```go
// 1. 定义 FSM
type DBFFSM struct {
    mu      sync.RWMutex
    cbf     *CountingBloomFilter
    shardID uint32
}

func (f *DBFFSM) Apply(log *raft.Log) interface{} {
    // 解析日志
    cmd := decodeCommand(log.Data)
    
    f.mu.Lock()
    defer f.mu.Unlock()
    
    switch cmd.Op {
    case OpAdd:
        f.cbf.Add(cmd.Key)
    case OpDelete:
        f.cbf.Delete(cmd.Key)
    }
    
    return nil
}

func (f *DBFFSM) Snapshot() (raft.FSMSnapshot, error) {
    // 创建快照
    return &DBFSnapshot{
        cbf:     f.cbf.Clone(),
        shardID: f.shardID,
    }, nil
}

func (f *DBFFSM) Restore(rc io.ReadCloser) error {
    // 从快照恢复
    defer rc.Close()
    
    snapshot := &DBFSnapshot{}
    if err := json.NewDecoder(rc).Decode(snapshot); err != nil {
        return err
    }
    
    f.mu.Lock()
    f.cbf = snapshot.cbf
    f.mu.Unlock()
    
    return nil
}

// 2. 初始化 Raft
func NewStorageNode(shardID uint32, dataDir string) (*StorageNode, error) {
    // 创建 FSM
    fsm := &DBFFSM{
        cbf:     NewCountingBloomFilter(1_000_000_000),
        shardID: shardID,
    }
    
    // 配置 Raft
    config := raft.DefaultConfig()
    config.LocalID = raft.ServerID(fmt.Sprintf("node-%d", shardID))
    config.HeartbeatTimeout = 1000 * time.Millisecond
    config.ElectionTimeout = 2000 * time.Millisecond
    config.LeaderLeaseTimeout = 1000 * time.Millisecond
    config.CommitTimeout = 50 * time.Millisecond
    config.SnapshotInterval = 300 * time.Second  // 5 分钟
    config.SnapshotThreshold = 10000             // 1 万条日志触发快照
    
    // 创建日志存储 (BoltDB)
    logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
    if err != nil {
        return nil, err
    }
    
    // 创建稳定存储
    stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "stable.db"))
    if err != nil {
        return nil, err
    }
    
    // 创建快照存储
    snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 3, os.Stderr)
    if err != nil {
        return nil, err
    }
    
    // 创建网络传输
    transport := raft.NewNetworkTransport(
        raft.NetworkTransportConfig{
            ServerAddressManager: nil,
            BindAddr:            "0.0.0.0",
            BindPort:            50052,
            MaxPool:             3,
            Timeout:             10 * time.Second,
        },
    )
    
    // 启动 Raft
    r, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
    if err != nil {
        return nil, err
    }
    
    // 配置集群 (首次启动)
    if isFirstBoot(dataDir) {
        configuration := raft.Configuration{
            Servers: []raft.Server{
                {
                    ID:       raft.ServerID("node-0"),
                    Address:  raft.ServerAddress("10.0.0.1:50052"),
                    Suffrage: raft.Voter,
                },
                {
                    ID:       raft.ServerID("node-1"),
                    Address:  raft.ServerAddress("10.0.0.2:50052"),
                    Suffrage: raft.Voter,
                },
                {
                    ID:       raft.ServerID("node-2"),
                    Address:  raft.ServerAddress("10.0.0.3:50052"),
                    Suffrage: raft.Voter,
                },
            },
        }
        future := r.BootstrapCluster(configuration)
        if err := future.Error(); err != nil {
            return nil, err
        }
    }
    
    return &StorageNode{
        raft:  r,
        fsm:   fsm,
        shardID: shardID,
    }, nil
}

// 3. 处理写入请求
func (n *StorageNode) Add(key string) error {
    // 检查是否为 Leader
    if n.raft.State() != raft.Leader {
        return ErrNotLeader
    }
    
    // 创建命令
    cmd := Command{
        Op:  OpAdd,
        Key: key,
    }
    
    // 提交到 Raft
    future := n.raft.Apply(encodeCommand(cmd), 10*time.Second)
    if err := future.Error(); err != nil {
        return err
    }
    
    // 等待响应
    if resp := future.Response(); resp != nil {
        if err, ok := resp.(error); ok {
            return err
        }
    }
    
    return nil
}

// 4. 处理读取请求
func (n *StorageNode) Contains(key string) bool {
    // Leader 读（直接从 FSM 读取）
    n.fsm.mu.RLock()
    defer n.fsm.mu.RUnlock()
    return n.fsm.cbf.Contains(key)
}
```

#### Raft 配置参数

```go
// 生产环境推荐配置
config := &raft.Config{
    // 选举相关
    HeartbeatTimeout:     1000 * time.Millisecond,  // 心跳超时
    ElectionTimeout:      2000 * time.Millisecond,  // 选举超时
    LeaderLeaseTimeout:   1000 * time.Millisecond,  // Leader 租约
    
    // 提交相关
    CommitTimeout:        50 * time.Millisecond,    // 提交超时
    
    // 快照相关
    SnapshotInterval:     300 * time.Second,        // 快照间隔 (5 分钟)
    SnapshotThreshold:    10000,                    // 快照阈值 (日志条数)
    TrailingLogs:         10000,                    // 保留日志数
    
    // 性能相关
    MaxAppendEntries:     64,                       // 批量追加日志数
    BatchApplyCh:         true,                     // 批量应用
    LogCacheSize:         512,                      // 日志缓存大小
    
    // 其他
    ShutdownOnRemove:     true,                     // 移除时关闭
    DisableBootstrap:     true,                     // 禁用自动引导
}
```

### 写入流程 (Add/Delete)

```
Client                    Leader (Node-0)              Followers (Node-1,2)
  │                            │                              │
  │  Add("key123")             │                              │
  │───────────────────────────▶│                              │
  │                            │                              │
  │                            │ 1. raft.Apply()              │
  │                            │    创建 LogEntry             │
  │                            │                              │
  │                            │ 2. Append to local BoltDB    │
  │                            │    (HashiCorp Raft 管理)      │
  │                            │                              │
  │                            │ 3. Parallel AppendEntries    │
  │                            │────────────────────────────▶│
  │                            │──────────────────────────────▶│
  │                            │                              │
  │                            │ 4. Ack (majority: 2/3)       │
  │                            │◀────────────────────────────│
  │                            │◀─────────────────────────────│
  │                            │                              │
  │                            │ 5. Commit                    │
  │                            │    FSM.Apply()               │
  │                            │    更新 CBF 计数器             │
  │                            │                              │
  │                            │ 6. Response to Client        │
  │◀───────────────────────────│                              │
  │  OK                        │                              │
  │                            │                              │
```

**详细步骤**:

1. **客户端请求**：API Gateway 路由 key 到对应分片的 Leader
2. **Raft 提交**：Leader 调用 `raft.Apply()` 提交命令
3. **日志追加**：HashiCorp Raft 自动追加日志到本地 BoltDB
4. **日志复制**：Raft 并行发送 `AppendEntries` 到所有 Followers
5. **多数派确认**：收到 > N/2 个确认后，操作提交
6. **应用 FSM**：调用 `FSM.Apply()` 更新 Counting Bloom Filter
7. **返回响应**：通知客户端成功

### 读取流程 (Contains)

```
Client                    Leader                      Followers
  │                         │                            │
  │  Contains("key123")     │                            │
  │────────────────────────▶│                            │
  │                         │                            │
  │                         │ 1. 直接从 FSM 读取            │
  │                         │    (无 Raft 开销)            │
  │                         │                            │
  │◀────────────────────────│                            │
  │  exists: true           │                            │
  │                         │                            │
```

**读取优化**:
- **Leader 读**：直接从 Leader 内存读取（低延迟）
- **线性化读**：可选，先 `raft.VerifyLeader()` 再读（强一致）
- **Follower 读**：允许从 Follower 读（可能短暂滞后）

默认采用 **Leader 读**，平衡性能和一致性。

### 故障恢复

#### Follower 故障

```
正常状态:
  Leader ──▶ Follower-1 (healthy)
         ──▶ Follower-2 (healthy)

Follower-1 宕机:
  Leader ──▶ Follower-1 (❌ no response)
         ──▶ Follower-2 (✅ healthy)

处理:
  1. Leader 检测到 Follower-1 心跳超时 (通过 Raft)
  2. 继续服务 (仍有 2/3 节点)
  3. K8s 自动重启 Follower-1 Pod
  4. Follower-1 从快照恢复 + 请求 Leader 同步缺失日志
  5. 重新加入集群 (通过 Gossip + Raft)
```

#### Leader 故障

```
正常状态:
  Leader (Node-0) ──▶ Follower-1
                  ──▶ Follower-2

Leader 宕机:
  1. Followers 检测不到心跳 (Raft HeartbeatTimeout)
  2. Follower-1 选举超时，成为 Candidate
  3. Follower-1 请求 Follower-2 投票 (HashiCorp Raft 自动处理)
  4. Follower-2 投票，Follower-1 成为新 Leader
  5. 更新 ConfigMap 中的分片映射表
  6. API Gateway 更新路由表，指向新 Leader
  7. K8s 重建 Node-0 Pod，作为 Follower 加入

总恢复时间: < 3 秒 (Raft 选举 + ConfigMap 更新)
```

### 脑裂处理

**场景**: 网络分区，集群分裂成两组

```
分区前:
  [Node-0(L), Node-1(F), Node-2(F), Node-3(F), Node-4(F), Node-5(F)]

分区后:
  分区 A: [Node-0, Node-1, Node-2]  (3 节点，有原 Leader)
  分区 B: [Node-3, Node-4, Node-5]  (3 节点，无 Leader)

处理:
  1. 分区 A: Node-0 仍是 Leader (有 3/6 节点，不满足多数派 4/6)
     - Node-0 检测到无法获得多数派，自动 step down
     - 分区 A 重新选举
  
  2. 分区 B: Node-3 发起选举，获得 3/6 票 (不满足多数派)
     - 无法选举成功，等待网络恢复
  
  3. 网络恢复后:
     - 重新选举 Leader
     - 同步日志
     - 集群合并

原则:
- 6 分片 × 3 副本 = 18 节点，多数派 = 10 票
- 单分片内：3 副本，多数派 = 2 票
- 只有获得多数派的分区能选举 Leader
```

---

## 持久化设计

### WAL (Write-Ahead Log) - AES-256 加密

**加密设计**:

```
┌─────────────────────────────────────────────────────────────┐
│                    WAL Encryption Flow                       │
│                                                              │
│  Application ──▶ Raft Log Entry ──▶ AES-256-GCM ──▶ Disk    │
│                      │                    │                  │
│                      │                    ▼                  │
│                      │            ┌─────────────────┐        │
│                      │            │  Key Derivation │        │
│                      │            │  (PBKDF2)       │        │
│                      │            └─────────────────┘        │
│                      │                    │                  │
│                      ▼                    ▼                  │
│            ┌─────────────────┐  ┌─────────────────┐         │
│            │  K8s Secret     │  │  Master Key     │         │
│            │  (Encryption Key)│  │  (per shard)    │         │
│            └─────────────────┘  └─────────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

**加密实现**:

```go
// 1. 密钥管理
type EncryptionKeyManager struct {
    mu        sync.RWMutex
    masterKey []byte  // AES-256 key (32 bytes)
    keyID     string  // 密钥版本 ID
}

func NewEncryptionKeyManager(namespace, shardID string) (*EncryptionKeyManager, error) {
    // 从 K8s Secret 加载密钥
    secret, err := k8sClient.CoreV1().Secrets(namespace).Get(
        context.TODO(),
        fmt.Sprintf("dbf-encryption-key-%s", shardID),
        metav1.GetOptions{},
    )
    if err != nil {
        return nil, err
    }
    
    key := secret.Data["master-key"]
    if len(key) != 32 {
        return nil, ErrInvalidKeySize
    }
    
    return &EncryptionKeyManager{
        masterKey: key,
        keyID:     string(secret.Data["key-id"]),
    }, nil
}

// 2. 加密 WAL 条目
type EncryptedLogEntry struct {
    KeyID     string    // 密钥版本 ID
    Nonce     []byte    // GCM nonce (12 bytes)
    Ciphertext []byte   // 加密后的数据
    Tag       []byte    // GCM authentication tag (16 bytes)
}

func (km *EncryptionKeyManager) Encrypt(data []byte) (*EncryptedLogEntry, error) {
    // 创建 AES-GCM cipher
    block, err := aes.NewCipher(km.masterKey)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // 生成随机 nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    // 加密
    ciphertext := gcm.Seal(nil, nonce, data, nil)
    
    return &EncryptedLogEntry{
        KeyID:      km.keyID,
        Nonce:      nonce,
        Ciphertext: ciphertext[:len(ciphertext)-16],  // 分离密文和 tag
        Tag:        ciphertext[len(ciphertext)-16:],
    }, nil
}

func (km *EncryptionKeyManager) Decrypt(entry *EncryptedLogEntry) ([]byte, error) {
    // 验证密钥版本
    if entry.KeyID != km.keyID {
        // 需要密钥轮换处理
        return nil, ErrKeyVersionMismatch
    }
    
    // 创建 AES-GCM cipher
    block, err := aes.NewCipher(km.masterKey)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // 解密
    ciphertext := append(entry.Ciphertext, entry.Tag...)
    plaintext, err := gcm.Open(nil, entry.Nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }
    
    return plaintext, nil
}

// 3. 集成到 Raft Log Store
type EncryptedBoltStore struct {
    bolt  *bolt.DB
    crypto *EncryptionKeyManager
}

func (e *EncryptedBoltStore) StoreLog(log *raft.Log) error {
    // 序列化日志
    data, err := json.Marshal(log)
    if err != nil {
        return err
    }
    
    // 加密
    encrypted, err := e.crypto.Encrypt(data)
    if err != nil {
        return err
    }
    
    // 存储到 BoltDB
    return e.bolt.Update(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte("logs"))
        key := fmt.Sprintf("%020d", log.Index)
        value, _ := json.Marshal(encrypted)
        return bucket.Put([]byte(key), value)
    })
}

func (e *EncryptedBoltStore) GetLog(index uint64, log *raft.Log) error {
    return e.bolt.View(func(tx *bolt.Tx) error {
        bucket := tx.Bucket([]byte("logs"))
        key := fmt.Sprintf("%020d", index)
        value := bucket.Get([]byte(key))
        
        if value == nil {
            return raft.ErrLogNotFound
        }
        
        // 反序列化
        var encrypted EncryptedLogEntry
        if err := json.Unmarshal(value, &encrypted); err != nil {
            return err
        }
        
        // 解密
        data, err := e.crypto.Decrypt(&encrypted)
        if err != nil {
            return err
        }
        
        return json.Unmarshal(data, log)
    })
}
```

**WAL 配置**:
- 加密算法：AES-256-GCM
- 密钥管理：K8s Secret（每分片独立密钥）
- 密钥轮换：支持多版本密钥，旧日志用旧密钥解密
- 单文件最大：100MB
- 滚动策略：达到大小限制或 5 分钟
- 清理：快照后删除旧 WAL

### Snapshot (快照) - 压缩 + 加密

```go
// 加密快照结构
type EncryptedSnapshot struct {
    Metadata struct {
        Term        uint64
        CommitIndex uint64
        Timestamp   int64
        ShardID     uint32
        KeyID       string    // 加密密钥版本
        Nonce       []byte    // GCM nonce
    }
    Data struct {
        Counters     []byte  // 加密后的计数器数组
        ElementCount int64
        Checksum     []byte  // SHA256 (加密前计算)
    }
}

// 创建加密快照
func (f *DBFFSM) createEncryptedSnapshot() (*EncryptedSnapshot, error) {
    // 1. 创建原始快照
    snapshot := &Snapshot{
        Metadata: {
            Term:        f.term,
            CommitIndex: f.commitIndex,
            Timestamp:   time.Now().Unix(),
            ShardID:     f.shardID,
        },
        Data: {
            Counters:     f.cbf.Serialize(),
            ElementCount: f.cbf.Count(),
        },
    }
    
    // 2. 计算校验和
    checksum := sha256.Sum256(snapshot.Data.Counters)
    snapshot.Data.Checksum = checksum[:]
    
    // 3. 压缩
    var compressed bytes.Buffer
    zw := zlib.NewWriter(&compressed)
    json.NewEncoder(zw).Encode(snapshot)
    zw.Close()
    
    // 4. 加密
    encrypted, err := f.keyManager.Encrypt(compressed.Bytes())
    if err != nil {
        return nil, err
    }
    
    // 5. 构建加密快照
    encSnapshot := &EncryptedSnapshot{
        Metadata: {
            Term:        snapshot.Metadata.Term,
            CommitIndex: snapshot.Metadata.CommitIndex,
            Timestamp:   snapshot.Metadata.Timestamp,
            ShardID:     snapshot.Metadata.ShardID,
            KeyID:       encrypted.KeyID,
            Nonce:       encrypted.Nonce,
        },
        Data: {
            Counters:     encrypted.Ciphertext,
            ElementCount: snapshot.Data.ElementCount,
            Checksum:     snapshot.Data.Checksum,
        },
    }
    
    return encSnapshot, nil
}
```

**快照触发条件**:
- 时间间隔：5 分钟
- WAL 大小：超过 500MB
- 日志条目数：超过 10 万条

**密钥轮换**:

```
1. 生成新密钥:
   - K8s Secret 更新 (dbf-encryption-key-shard-0)
   - key-id 递增 (v1 → v2)
   - 滚动更新 Storage Node Pod

2. 密钥轮换期间:
   - 新 WAL 条目：用新密钥 (v2) 加密
   - 旧 WAL 条目：保留旧密钥 (v1) 标识
   - 解密时：根据 key-id 选择对应密钥

3. 旧密钥清理:
   - 当所有旧 WAL 被快照替代后
   - 可安全删除旧密钥
   - 保留至少 2 个版本用于回滚
```

---

## 数据分片与复制

### 分片策略（6 分片）

```
一致性 Hash 环 (2^32 空间)

分片 0: [0x00000000, 0x2A000000]         (0 - 715827882)
分片 1: [0x2A000001, 0x55000000]         (715827883 - 1431655765)
分片 2: [0x55000001, 0x7FFFFFFF]         (1431655766 - 2147483647)
分片 3: [0x80000000, 0xAAAAAAAA]         (2147483648 - 2863311530)
分片 4: [0xAAAAAAAA+1, 0xD5000000]       (2863311531 - 3579139413)
分片 5: [0xD5000001, 0xFFFFFFFF]         (3579139414 - 4294967295)

每个分片负责 1/6 的 Hash 空间 (约 7.15 亿个 hash 值)
```

**分片计算**:
```go
func getShardID(key string, shardCount int) uint32 {
    hash := murmur3.Sum32([]byte(key))
    return hash % uint32(shardCount)  // shardCount = 6
}
```

### Hash 库选择

使用 **github.com/twmb/murmur3**：

```go
import "github.com/twmb/murmur3"

func hashKey(key string) uint32 {
    return murmur3.Sum32([]byte(key))
}

// 优势:
// - 高性能 (比 MurmurHash2 快 20-30%)
// - 低碰撞率
// - Go 语言原生实现
// - 活跃维护 (twmb 团队)
// - 与 Java MurmurHash3 兼容
```

### 复制组（6 分片 × 3 副本）

```
分片 0: [Node-0 (L), Node-1 (F), Node-2 (F)]
分片 1: [Node-3 (L), Node-4 (F), Node-5 (F)]
分片 2: [Node-6 (L), Node-7 (F), Node-8 (F)]
分片 3: [Node-9 (L), Node-10 (F), Node-11 (F)]
分片 4: [Node-12 (L), Node-13 (F), Node-14 (F)]
分片 5: [Node-15 (L), Node-16 (F), Node-17 (F)]

交叉复制策略:
- 同一分片的 3 副本部署在不同 Node/Zone
- 避免单点故障
- 支持跨可用区容灾
```

---

## 监控与可观测性

### Prometheus Metrics

```promql
# Raft 状态 (HashiCorp Raft)
dbf_raft_state{shard="0", state="leader|follower|candidate"}
dbf_raft_term{shard="0"}
dbf_raft_last_log_index{shard="0"}
dbf_raft_commit_index{shard="0"}
dbf_raft_last_snapshot_index{shard="0"}

# 日志复制
dbf_raft_replication_lag{shard="0", follower="node-1"}
dbf_raft_append_entries_latency_seconds{shard="0", quantile="0.99"}

# 加密指标
dbf_encryption_operations_total{type="encrypt|decrypt", shard="0"}
dbf_encryption_latency_seconds{type="encrypt", shard="0"}
dbf_encryption_key_version{shard="0"}  # 当前密钥版本

# Gossip 指标
dbf_gossip_members_count{shard="0"}
dbf_gossip_messages_sent_total{shard="0"}
dbf_gossip_messages_received_total{shard="0"}

# ConfigMap 指标
dbf_configmap_watch_events_total{type="update|error"}
dbf_configmap_last_update_timestamp{}

# 操作指标
dbf_operations_total{type="add|delete|contains", shard="0"}
dbf_operation_duration_seconds{type="add", quantile="0.99"}

# 数据指标
dbf_element_count{shard="0"}
dbf_memory_usage_bytes{shard="0"}

# 持久化指标
dbf_wal_size_bytes{shard="0"}
dbf_wal_encrypted_size_bytes{shard="0"}
dbf_snapshot_age_seconds{shard="0"}
dbf_wal_write_latency_seconds{shard="0"}

# 错误指标
dbf_errors_total{type="replication|election|io|encryption", shard="0"}
```

### 日志

结构化日志 (JSON 格式):
```json
{
  "timestamp": "2026-03-11T03:38:00Z",
  "level": "info",
  "node": "dbf-storage-0",
  "shard": 0,
  "event": "leader_elected",
  "term": 5,
  "votes": 2,
  "raft_library": "hashicorp/raft"
}
```

---

## 性能指标（6 分片）

### 内存分配

```
单分片容量：10 亿元素
计数器大小：4 bit × 2 (double hash) = 8 bit = 1 byte/元素
单分片内存：10 亿 × 1 byte = 1GB

6 分片总内存：6GB (数据平面)
每 Pod 内存：1.5GB (单分片副本) + 0.5GB (Raft + 缓冲) = 2Gi

总 Pod 数：18 (6 分片 × 3 副本)
总内存需求：18 × 2Gi = 36Gi
```

### QPS 分配

```
目标总 QPS: 60,000 QPS

每分片 QPS: 60,000 / 6 = 10,000 QPS
每 Leader QPS: 10,000 QPS (写入)
每 Follower QPS: 10,000 QPS (日志复制)

单 Pod 峰值 QPS: 20,000 QPS (同时担任 Leader + 其他分片 Follower)

写入延迟分解:
- Raft 日志追加：2ms
- 网络复制 (RTT): 5ms
- 多数派确认：10ms
- FSM 应用：1ms
- 总延迟：18ms (P99)

读取延迟:
- Leader 读：< 1ms (内存操作)
- 线性化读：~5ms (需验证 Leader)
```

### 存储需求

```
单分片 WAL:
- 单条日志：~100 bytes (加密后 ~150 bytes)
- 每秒 10,000 操作：1.5MB/s
- 5 分钟快照间隔：450MB
- 单 WAL 文件上限：100MB (滚动)

单分片快照:
- 10 亿计数器：1GB (压缩后 ~400MB)
- 加密开销：+16 bytes/块

总存储 (18 Pod):
- WAL: 18 × 100MB × 5 (保留 5 个文件) = 9GB
- 快照：18 × 400MB × 3 (保留 3 个) = 21.6GB
- 总计：~31GB

PV 配置：每 Pod 10Gi (共 180Gi)
```

---

## 部署配置

详见 [deploy/k8s/](deploy/k8s/) 目录

- `statefulset.yaml` - Storage Node StatefulSet (18 副本)
- `deployment.yaml` - API Gateway Deployment
- `service.yaml` - Headless Service + LoadBalancer
- `configmap.yaml` - 集群元数据 ConfigMap
- `secret.yaml` - 加密密钥 (AES-256)
- `pvc.yaml` - 持久化卷声明

---

## 变更摘要

### 主要变更 (2026-03-11)

1. **分片数量**: 3 → 6
   - 重新计算内存分配：单分片 1GB，总内存 6GB
   - 重新计算 QPS 分配：每分片 10,000 QPS
   - Pod 总数：6 → 18 (6 分片 × 3 副本)

2. **元数据服务**: etcd → K8s ConfigMap + Gossip
   - 移除 etcd 依赖
   - 使用 ConfigMap 存储集群元数据
   - 使用 HashiCorp Memberlist 实现 Gossip 协议
   - 本地缓存 + K8s Watch API 实时更新

3. **Raft 实现**: 自研 → HashiCorp Raft
   - 集成 HashiCorp Raft 库
   - 使用 BoltDB 作为日志存储
   - 实现 FSM 接口管理 CBF 状态
   - 配置优化：选举超时 2s，快照间隔 5 分钟

4. **WAL 加密**: 新增 AES-256-GCM 加密
   - 密钥管理：K8s Secret (每分片独立密钥)
   - 加密算法：AES-256-GCM
   - 支持密钥轮换
   - 快照同样加密存储

5. **Hash 库**: 确认使用 github.com/twmb/murmur3
   - 高性能 MurmurHash3 实现
   - 与 Java 版本兼容
   - 活跃维护

### 影响评估

- ✅ 性能：6 分片提升并行度，理论 QPS 提升 2 倍
- ✅ 可靠性：HashiCorp Raft 成熟稳定，降低自研风险
- ✅ 运维：移除 etcd 简化架构，降低运维复杂度
- ✅ 安全：WAL 加密满足数据加密要求
- ⚠️ 资源：Pod 数量从 6 增至 18，内存需求从 12Gi 增至 36Gi
- ⚠️ 延迟：Raft 库集成可能增加 2-3ms 延迟（可接受）

---

## 参考资料

- [HashiCorp Raft](https://github.com/hashicorp/raft)
- [HashiCorp Memberlist](https://github.com/hashicorp/memberlist)
- [MurmurHash3 (twmb)](https://github.com/twmb/murmur3)
- [AES-GCM 加密](https://pkg.go.dev/crypto/cipher)
- [K8s ConfigMap](https://kubernetes.io/docs/concepts/configuration/configmap/)
- [K8s Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
