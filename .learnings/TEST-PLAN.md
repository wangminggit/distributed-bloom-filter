# 测试计划 (TEST-PLAN.md)

**版本**: 1.0  
**创建时间**: 2026-03-13  
**负责人**: Sarah Liu (高级测试工程师)  
**里程碑**: M5 测试完成

---

## 📋 目录

1. [测试策略](#测试策略)
2. [覆盖率目标](#覆盖率目标)
3. [测试用例清单](#测试用例清单)
4. [集成测试框架](#集成测试框架)
5. [性能测试计划](#性能测试计划)
6. [故障注入测试](#故障注入测试)
7. [测试执行计划](#测试执行计划)

---

## 🎯 测试策略

### 单元测试策略

**目标**: 验证每个函数/方法的最小可测试单元

**原则**:
- 每个公共函数至少一个测试用例
- 边界条件必须测试 (空值、最大值、最小值)
- 错误路径必须测试
- 使用表驱动测试 (Table-Driven Tests) 提高可维护性
- 测试名称格式：`TestFunctionName_Scenario_ExpectedResult`

**工具**:
- Go 原生 `testing` 包
- `testify/assert` 和 `testify/require` 用于断言
- `go test -cover` 用于覆盖率统计
- `go test -race` 用于竞态检测

**覆盖率要求**:
- 核心模块 (pkg/bloom, internal/wal): >85%
- 服务模块 (internal/grpc): >80%
- 入口模块 (cmd/server): >70%

---

### 集成测试策略

**目标**: 验证模块间交互和端到端功能

**测试层级**:
```
┌─────────────────────────────────────┐
│         E2E 测试 (黑盒)              │
│   - 完整工作流程验证                  │
│   - 多节点集群测试                    │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│       集成测试 (灰盒)                │
│   - gRPC 服务 + Raft 共识            │
│   - WAL 持久化 + 恢复                │
│   - 元数据服务集成                    │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│       组件测试 (白盒)                │
│   - 单模块功能验证                    │
│   - Mock 外部依赖                    │
└─────────────────────────────────────┘
```

**测试数据管理**:
- 使用 `t.TempDir()` 创建临时目录
- 测试后自动清理
- 使用固定种子生成可重复测试数据

**依赖隔离**:
- 使用接口 + Mock 隔离外部依赖
- 使用 testcontainers 运行真实依赖 (可选)

---

### 性能测试策略

**目标**: 验证系统达到 10 万 QPS, P99 <5ms

**测试类型**:
1. **基准测试**: 单接口性能基线
2. **负载测试**: 预期负载下稳定性
3. **压力测试**: 极限负载下行为
4. **耐久测试**: 长时间运行稳定性

**工具选型**:
- `wrk`: HTTP/gRPC 压测
- `vegeta`: 持续负载测试
- Go `testing.B`: 单元基准测试

**监控指标**:
- QPS (每秒请求数)
- 延迟分布 (P50, P90, P95, P99)
- 错误率
- CPU/内存使用率
- Raft 日志复制延迟

---

### 安全测试策略

**目标**: 验证安全机制有效性

**测试范围**:
1. **认证测试**:
   - 无效 API Key 拒绝
   - 过期时间戳拒绝
   - 无效签名拒绝
   - 重放攻击防护

2. **授权测试**:
   - 未认证请求拒绝
   - 权限不足拒绝

3. **传输安全**:
   - TLS 握手成功
   - 明文连接拒绝
   - 证书验证

4. **限流测试**:
   - 超出速率限制拒绝
   - 限流恢复验证

5. **输入验证**:
   - SQL 注入防护
   - 缓冲区溢出防护
   - 特殊字符处理

---

## 📊 覆盖率目标

### 模块覆盖率目标

| 模块 | 当前 | 目标 | 优先级 |
|------|------|------|--------|
| pkg/bloom/ | 74.1% | 85% | P0 |
| internal/wal/ | 59.8% | 85% | P0 |
| internal/grpc/ | 60.4% | 85% | P0 |
| cmd/server/ | 0.0% | 70% | P1 |
| internal/metadata/ | 0.0% | 70% | P1 |
| internal/raft/ | N/A | 80% | P0 |

### 关键路径 100% 覆盖

以下路径必须 100% 覆盖:

1. **核心数据路径**:
   - bloom.Add() → 成功/失败
   - bloom.Contains() → true/false
   - bloom.Remove() → 成功/失败

2. **安全关键路径**:
   - AuthInterceptor.UnaryInterceptor() → 认证通过/拒绝
   - RateLimitInterceptor.UnaryInterceptor() → 允许/限流
   - TLS 配置加载 → 成功/失败

3. **持久化路径**:
   - WALWriter.Write() → 成功/滚动
   - WALReader.ReadAll() → 成功/失败
   - Encryptor.Encrypt/Decrypt() → 往返验证

4. **共识路径**:
   - RaftNode.Start() → Leader/Follower
   - RaftNode.Apply() → 日志复制
   - RaftNode.Shutdown() → 优雅关闭

---

## ✅ 测试用例清单

### pkg/bloom/ 测试用例

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| B001 | NewCountingBloomFilter 正常初始化 | ✅ | P0 |
| B002 | NewCountingBloomFilter m=0 边界 | ❌ | P1 |
| B003 | NewCountingBloomFilter k=0 边界 | ❌ | P1 |
| B004 | Add 正常添加 | ✅ | P0 |
| B005 | Add nil item | ❌ | P1 |
| B006 | Add 计数器溢出 | ✅ | P0 |
| B007 | Contains 存在元素 | ✅ | P0 |
| B008 | Contains 不存在元素 | ✅ | P0 |
| B009 | Remove 存在元素 | ✅ | P0 |
| B010 | Remove 不存在元素 | ❌ | P2 |
| B011 | Count 多次添加 | ✅ | P0 |
| B012 | Reset 清空过滤器 | ✅ | P1 |
| B013 | Serialize/Deserialize 正常往返 | ✅ | P0 |
| B014 | Deserialize 数据过短 | ✅ | P0 |
| B015 | Deserialize 数据损坏 | ❌ | P1 |
| B016 | Deserialize 超大过滤器 | ✅ | P0 |
| B017 | Deserialize 无效 k 值 | ✅ | P0 |
| B018 | 并发 Add 操作 | ✅ | P0 |
| B019 | 并发 Add/Contains 混合 | ❌ | P1 |
| B020 | 哈希索引确定性 | ✅ | P1 |
| B021 | 哈希索引范围验证 | ✅ | P1 |
| B022 | 假阳性率测试 | ✅ | P2 |

**新增测试用例** (优先级排序):
1. `TestNewCountingBloomFilter_EdgeCases` - B002, B003
2. `TestAdd_NilItem` - B005
3. `TestRemove_NonExistentItem` - B010
4. `TestDeserialize_CorruptedData` - B015
5. `TestConcurrency_MixedOperations` - B019

---

### internal/wal/ 测试用例

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| W001 | Encryptor 加密解密往返 | ✅ | P0 |
| W002 | Encryptor 密钥轮换 | ✅ | P0 |
| W003 | Encryptor 错误密钥解密 | ❌ | P1 |
| W004 | WALWriter 正常写入 | ✅ | P0 |
| W005 | WALWriter 文件滚动 | ✅ | P0 |
| W006 | WALWriter 滚动边界条件 | ❌ | P1 |
| W007 | WALWriter 并发写入 | ❌ | P1 |
| W008 | WALWriter 磁盘空间不足 | ❌ | P2 |
| W009 | WALReader 正常读取 | ✅ | P0 |
| W010 | WALReader 文件损坏 | ❌ | P1 |
| W011 | WALReader 空目录 | ❌ | P2 |
| W012 | WAL 恢复测试 | ✅ | P0 |
| W013 | K8sSecretLoader 正常加载 | ✅ | P1 |
| W014 | K8sSecretLoader 文件缺失 | ❌ | P2 |

**新增测试用例**:
1. `TestEncryptor_WrongKey` - W003
2. `TestWALWriter_RollingBoundary` - W006
3. `TestWALWriter_ConcurrentWrites` - W007
4. `TestWALReader_CorruptedFile` - W010
5. `TestWALReader_EmptyDirectory` - W011
6. `TestK8sSecretLoader_MissingFile` - W014

---

### internal/grpc/ 测试用例

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| G001 | AuthInterceptor 有效认证 | ✅ | P0 |
| G002 | AuthInterceptor 缺失认证 | ✅ | P0 |
| G003 | AuthInterceptor 无效 API Key | ✅ | P0 |
| G004 | AuthInterceptor 过期时间戳 | ✅ | P0 |
| G005 | AuthInterceptor 无效签名 | ✅ | P0 |
| G006 | AuthInterceptor 边界时间戳 | ❌ | P1 |
| G007 | RateLimitInterceptor 限制内 | ✅ | P0 |
| G008 | RateLimitInterceptor 超限制 | ✅ | P0 |
| G009 | RateLimitInterceptor 令牌恢复 | ❌ | P1 |
| G010 | StreamInterceptor 认证 | ❌ | P1 |
| G011 | TLS 配置加载 | ❌ | P0 |
| G012 | TLS 握手成功 | ❌ | P0 |
| G013 | TLS 证书验证失败 | ❌ | P1 |
| G014 | Server.Add 正常添加 | ✅ | P0 |
| G015 | Server.Add 空 item | ✅ | P0 |
| G016 | Server.Add nil item | ✅ | P0 |
| G017 | Server.Contains 存在 | ✅ | P0 |
| G018 | Server.Contains 不存在 | ✅ | P0 |
| G019 | Server.BatchAdd 批量添加 | ✅ | P0 |
| G020 | Server.BatchAdd 部分失败 | ✅ | P0 |
| G021 | Server.Remove 正常删除 | ✅ | P0 |
| G022 | Server.GetStats 统计信息 | ✅ | P0 |
| G023 | Server 并发请求 | ❌ | P1 |
| G024 | Server Raft 失败处理 | ❌ | P1 |
| G025 | MemoryAPIKeyStore 正常 | ✅ | P0 |
| G026 | GetClientIP 空上下文 | ✅ | P2 |

**新增测试用例**:
1. `TestAuthInterceptor_BoundaryTimestamp` - G006
2. `TestRateLimitInterceptor_TokenRecovery` - G009
3. `TestStreamInterceptor_Auth` - G010
4. `TestTLSConfiguration` - G011
5. `TestTLSHandshake` - G012
6. `TestTLS_CertVerificationFailed` - G013
7. `TestServer_ConcurrentRequests` - G023
8. `TestServer_RaftFailure` - G024

---

### cmd/server/ 测试用例 (新增)

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| S001 | main 正常启动 | ❌ | P0 |
| S002 | main 配置加载失败 | ❌ | P1 |
| S003 | main 端口占用处理 | ❌ | P1 |
| S004 | config 加载有效配置 | ❌ | P0 |
| S005 | config 加载无效配置 | ❌ | P0 |
| S006 | config 默认值 | ❌ | P1 |
| S007 | signal SIGINT 优雅关闭 | ❌ | P1 |
| S008 | signal SIGTERM 优雅关闭 | ❌ | P1 |

---

### internal/metadata/ 测试用例 (新增)

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| M001 | Service.Get 正常获取 | ❌ | P0 |
| M002 | Service.Set 正常设置 | ❌ | P0 |
| M003 | Service.Delete 正常删除 | ❌ | P1 |
| M004 | Service 并发访问 | ❌ | P1 |
| M005 | Service 持久化 | ❌ | P1 |
| M006 | Service 恢复 | ❌ | P1 |

---

### internal/raft/ 测试用例 (待实现)

| ID | 测试用例 | 状态 | 优先级 |
|----|----------|------|--------|
| R001 | Node.Start Leader 选举 | ❌ | P0 |
| R002 | Node.Start Follower 加入 | ❌ | P0 |
| R003 | Node.Apply 日志复制 | ❌ | P0 |
| R004 | Node.Apply 多数派确认 | ❌ | P0 |
| R005 | Node.Shutdown 优雅关闭 | ❌ | P1 |
| R006 | Node 网络分区恢复 | ❌ | P1 |
| R007 | Node Leader 故障转移 | ❌ | P0 |

---

## 🧪 集成测试框架

### 测试框架选型

**推荐**: `testify` + Go 原生 testing

**理由**:
- 社区广泛使用，文档完善
- 与 Go 原生 testing 无缝集成
- 提供丰富的断言方法
- 支持 Mock 和 Suite

**安装**:
```bash
go get github.com/stretchr/testify
```

### 测试数据结构

```go
// 测试配置
type TestConfig struct {
    ServerAddr string
    Timeout    time.Duration
    DataDir    string
}

// 测试客户端
type TestClient struct {
    conn   *grpc.ClientConn
    client pb.DBFServiceClient
    ctx    context.Context
}

// 测试集群
type TestCluster struct {
    nodes    []*raft.Node
    servers  []*grpc.DBFServer
    clients  []*TestClient
    dataDirs []string
}
```

### Mock 策略

**原则**:
- 只 Mock 外部依赖 (数据库、网络、文件系统)
- 不 Mock 被测代码
- 使用接口隔离依赖

**示例**:
```go
// 定义接口
type RaftNode interface {
    IsLeader() bool
    Apply(command []byte) error
    Shutdown() error
}

// Mock 实现
type MockRaftNode struct {
    isLeader bool
    applyFunc func([]byte) error
}

func (m *MockRaftNode) IsLeader() bool {
    return m.isLeader
}

func (m *MockRaftNode) Apply(command []byte) error {
    if m.applyFunc != nil {
        return m.applyFunc(command)
    }
    return nil
}
```

---

## 🚀 性能测试计划

### 压测工具

**主工具**: `wrk`

**安装**:
```bash
git clone https://github.com/wg/wrk.git
cd wrk && make
sudo cp wrk /usr/local/bin/
```

**备选**: `vegeta` (持续负载测试)

### 性能基准

| 指标 | 目标 | 警告 | 严重 |
|------|------|------|------|
| QPS | 100,000 | <80,000 | <50,000 |
| P50 延迟 | <1ms | >2ms | >5ms |
| P95 延迟 | <3ms | >5ms | >10ms |
| P99 延迟 | <5ms | >10ms | >20ms |
| 错误率 | <0.1% | >1% | >5% |
| CPU 使用率 | <70% | >80% | >90% |
| 内存使用率 | <50% | >70% | >85% |

### 测试场景设计

#### 场景 1: 单节点基准测试

**目的**: 建立性能基线

**配置**:
- 1 节点
- 100 并发连接
- 持续 60 秒

**wrk 命令**:
```bash
wrk -t12 -c100 -d60s --latency http://localhost:8080/add
```

#### 场景 2: 三节点集群测试

**目的**: 验证 Raft 共识对性能影响

**配置**:
- 3 节点集群
- 100 并发连接
- 持续 60 秒
- 请求发送到 Leader

**预期**:
- QPS 下降 <30% (对比单节点)
- P99 延迟增加 <50%

#### 场景 3: 混合读写负载

**目的**: 模拟真实场景

**配置**:
- 80% 读 (Contains)
- 20% 写 (Add)
- 100 并发连接
- 持续 300 秒

#### 场景 4: 批量操作测试

**目的**: 验证批量接口性能优势

**配置**:
- BatchAdd (100 items/batch)
- 50 并发连接
- 持续 60 秒

**预期**:
- 吞吐量提升 >5x (对比单条 Add)

#### 场景 5: 耐久测试

**目的**: 验证长时间运行稳定性

**配置**:
- 3 节点集群
- 50 并发连接
- 持续 24 小时
- 监控内存泄漏

### 性能测试脚本

```bash
#!/bin/bash
# tests/performance/run-benchmark.sh

set -e

echo "=== DBF Performance Benchmark ==="

# Start server
./bin/start-server.sh &
SERVER_PID=$!
sleep 5

# Warm up
echo "Warming up..."
wrk -t4 -c20 -d10s http://localhost:8080/add

# Run benchmark
echo "Running benchmark..."
wrk -t12 -c100 -d60s --latency http://localhost:8080/add > results/benchmark.txt

# Cleanup
kill $SERVER_PID

echo "Benchmark complete. Results: results/benchmark.txt"
```

---

## 💥 故障注入测试

### 测试框架

**推荐**: `chaos-mesh` (K8s 环境) 或自定义故障注入

**测试类型**:
1. 网络分区
2. 节点故障
3. 磁盘故障
4. CPU/内存压力

### 网络分区测试

**目的**: 验证 Raft 在网络分区下的行为

**场景**:
1. **Leader 隔离**: Leader 与 Follower 网络断开
   - 预期: 新 Leader 选举，旧 Leader 降级
   - 恢复: 网络恢复后旧 Leader 重新加入

2. **Follower 隔离**: 一个 Follower 被隔离
   - 预期: 集群继续工作 (多数派仍在)
   - 恢复: 网络恢复后数据同步

3. **多数派隔离**: 多数节点被隔离
   - 预期: 集群不可写 (保护数据一致性)
   - 恢复: 网络恢复后自动恢复

**测试脚本**:
```bash
#!/bin/bash
# tests/chaos/network-partition.sh

# Partition Leader from Followers
iptables -A OUTPUT -d $FOLLOWER1_IP -j DROP
iptables -A OUTPUT -d $FOLLOWER2_IP -j DROP

sleep 30

# Verify new leader elected
# ...

# Restore network
iptables -D OUTPUT -d $FOLLOWER1_IP -j DROP
iptables -D OUTPUT -d $FOLLOWER2_IP -j DROP
```

### 节点故障测试

**目的**: 验证节点崩溃恢复

**场景**:
1. **Leader 崩溃**: kill -9 Leader 进程
   - 预期: Follower 检测到并选举新 Leader (<500ms)
   - 数据: 无数据丢失

2. **Follower 崩溃**: kill -9 Follower 进程
   - 预期: 集群继续工作
   - 恢复: 重启后数据同步

3. **同时崩溃 2 节点**: 3 节点集群中崩溃 2 个
   - 预期: 集群不可用 (保护一致性)
   - 恢复: 重启后自动恢复

**测试脚本**:
```bash
#!/bin/bash
# tests/chaos/node-failure.sh

# Kill Leader
LEADER_PID=$(pgrep -f "dbf-server.*leader")
kill -9 $LEADER_PID

# Wait for new leader election
sleep 5

# Verify new leader
# ...

# Restart old leader
./bin/dbf-server --config=config1.yaml &
```

### 数据恢复测试

**目的**: 验证 WAL 和快照恢复

**场景**:
1. **正常关闭恢复**: 优雅关闭后重启
   - 预期: 数据完整恢复

2. **崩溃恢复**: kill -9 后重启
   - 预期: WAL 回放，数据恢复

3. **快照恢复**: 删除 WAL，从快照恢复
   - 预期: 快照数据恢复

4. **损坏恢复**: 损坏部分 WAL 文件
   - 预期: 跳过损坏部分，恢复可用数据

**测试脚本**:
```bash
#!/bin/bash
# tests/chaos/data-recovery.sh

# Add test data
for i in {1..1000}; do
    curl -X POST http://localhost:8080/add -d "item-$i"
done

# Crash server
kill -9 $(pgrep -f dbf-server)

# Restart
./bin/dbf-server --config=config.yaml &
sleep 5

# Verify data
for i in {1..1000}; do
    curl http://localhost:8080/contains?item=item-$i
done
```

---

## 📅 测试执行计划

### Week 1 (2026-03-13 ~ 2026-03-19)

**目标**: 修复构建错误，补充基础测试

| 日期 | 任务 | 负责人 | 状态 |
|------|------|--------|------|
| 03-13 | 测试覆盖率分析 | Sarah | ✅ |
| 03-13 | 创建测试计划文档 | Sarah | ✅ |
| 03-14 | 修复 internal/raft/ 构建错误 | David | ⏳ |
| 03-15 | 补充 pkg/bloom/ 边界测试 | David | ⏳ |
| 03-16 | 补充 internal/wal/ 错误处理测试 | David | ⏳ |
| 03-17 | 创建 cmd/server/ 基础测试 | David | ⏳ |
| 03-18 | 创建 internal/metadata/ 基础测试 | David | ⏳ |
| 03-19 | 周报复盘 | 全员 | ⏳ |

**里程碑**: 整体覆盖率 >60%

---

### Week 2 (2026-03-20 ~ 2026-03-26)

**目标**: 完善安全测试，修复 race detection

| 日期 | 任务 | 负责人 | 状态 |
|------|------|--------|------|
| 03-20 | 补充 internal/grpc/ TLS 测试 | David | ⏳ |
| 03-21 | 补充 internal/grpc/ 并发测试 | David | ⏳ |
| 03-22 | 修复 race detection 问题 | David | ⏳ |
| 03-23 | 创建安全测试用例 | Sarah | ⏳ |
| 03-24 | 执行安全测试 | Sarah | ⏳ |
| 03-25 | 补充集成测试用例 | Sarah | ⏳ |
| 03-26 | 周报复盘 | 全员 | ⏳ |

**里程碑**: 整体覆盖率 >75%, race detection 通过

---

### Week 3 (2026-03-27 ~ 2026-04-02)

**目标**: 性能测试和故障注入测试

| 日期 | 任务 | 负责人 | 状态 |
|------|------|--------|------|
| 03-27 | 搭建性能测试环境 | Sarah | ⏳ |
| 03-28 | 执行基准测试 | Sarah | ⏳ |
| 03-29 | 执行负载测试 | Sarah | ⏳ |
| 03-30 | 执行压力测试 | Sarah | ⏳ |
| 03-31 | 执行故障注入测试 | Sarah | ⏳ |
| 04-01 | 性能优化 (如有需要) | David | ⏳ |
| 04-02 | 周报复盘 | 全员 | ⏳ |

**里程碑**: 性能测试达标 (10 万 QPS, P99 <5ms)

---

### Week 4 (2026-04-03 ~ 2026-04-09)

**目标**: 测试收尾，M5 验收

| 日期 | 任务 | 负责人 | 状态 |
|------|------|--------|------|
| 04-03 | 补充缺失测试用例 | David | ⏳ |
| 04-04 | 最终覆盖率检查 | Sarah | ⏳ |
| 04-05 | 生成测试报告 | Sarah | ⏳ |
| 04-06 | M5 验收评审 | 全员 | ⏳ |
| 04-07 | 修复验收问题 | David | ⏳ |
| 04-08 | 最终验证 | Sarah | ⏳ |
| 04-09 | M5 里程碑完成 | 全员 | ⏳ |

**里程碑**: M5 测试完成，所有指标达标

---

## 📊 测试报告模板

### 每日测试报告

```markdown
## 测试日报 - YYYY-MM-DD

### 今日执行测试
- 单元测试：X 个
- 集成测试：Y 个
- 性能测试：Z 个

### 测试结果
- 通过：A 个
- 失败：B 个
- 跳过：C 个

### 覆盖率变化
- 当前覆盖率：X%
- 较昨日：+/-Y%

### 发现问题
1. [严重] 问题描述
2. [一般] 问题描述

### 明日计划
- 测试用例 1
- 测试用例 2
```

### 周测试报告

```markdown
## 测试周报 - Week N

### 本周概览
- 新增测试用例：X 个
- 修复 Bug: Y 个
- 覆盖率提升：+Z%

### 模块覆盖率
| 模块 | 周初 | 周末 | 变化 |
|------|------|------|------|
| pkg/bloom/ | X% | Y% | +/-Z% |

### 性能测试结果
- QPS: X (目标：100,000)
- P99: Xms (目标：<5ms)

### 风险和问题
1. 问题描述，影响，缓解措施

### 下周计划
- 重点任务 1
- 重点任务 2
```

---

## ✅ 验收标准

### M5 里程碑验收清单

- [ ] 单元测试覆盖率 >80% (所有模块)
- [ ] 关键路径 100% 覆盖
- [ ] race detection 测试通过
- [ ] 集成测试全部通过
- [ ] 性能测试达标 (10 万 QPS, P99 <5ms)
- [ ] 故障注入测试通过
- [ ] 安全测试通过
- [ ] 测试文档完整
- [ ] 测试脚本可重复执行

---

*文档维护：Sarah Liu*  
*最后更新：2026-03-13*
