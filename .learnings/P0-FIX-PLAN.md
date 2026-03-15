# P0 问题修复计划 (2026-03-14)

**创建时间**: 2026-03-14 13:05  
**负责人**: Guawa (PM) + David (开发)  
**优先级**: 🔴 P0 (必须修复)  
**截止日期**: 2026-03-15

---

## 📋 P0 问题清单

### 1. Raft FSM 数据竞争 ✅ 已完成
**来源**: 代码评审  
**风险**: 状态不一致  
**位置**: `internal/raft/node.go`, `fsm.go`  
**问题**: 存在两个 Apply 实现 (Node.Apply 和 BloomFSM.Apply)  
**修复方案**: 
- 在 Node 中嵌入 BloomFSM 实例
- Node.Apply() 委托给 BloomFSM.Apply() 执行
- Snapshot/Restore 也委托给 BloomFSM
- 确保所有 FSM 状态变更通过单一路径
**负责人**: David  
**完成时间**: 2026-03-14 13:10  
**验证**: go test -race ./internal/raft/... -count=2 (无数据竞争)

---

### 2. gRPC 拦截器覆盖
**来源**: 代码评审  
**风险**: 安全功能可能失效  
**问题**: 多次调用 `UnaryInterceptor()` 导致只有最后一个生效  
**修复方案**: 实现拦截器链式调用，确保所有拦截器生效  
**负责人**: David  
**预计时间**: 1 小时  
**状态**: ✅ 已完成  
**完成时间**: 2026-03-14 13:15  
**修复详情**: 
- 修改 `internal/grpc/server.go` 使用 `grpc.ChainUnaryInterceptor()` 串联所有拦截器
- 拦截器执行顺序：认证 → 限流
- 同时使用 `grpc.ChainStreamInterceptor()` 处理流式请求
- 添加测试 `TestChainedInterceptors` 验证多个拦截器同时生效
**测试结果**: 所有拦截器测试通过 (TestAuthInterceptor, TestRateLimitInterceptor, TestChainedInterceptors)

---

### 3. WAL 死锁风险 ✅ 已完成
**来源**: 代码评审  
**风险**: 服务挂起  
**位置**: `internal/wal/encryptor.go`  
**问题**: `rollFile()` 和 `rollFileLocked()` 调用约定不明确，可能导致死锁  
**修复方案**: 
- 移除未使用的公共 `rollFile()` 方法，消除死锁风险
- 重命名 `rollFileLocked()` → `doRollFile()`，明确内部方法约定
- 添加详细的锁约定注释到 `WALWriter` 结构体
- 更新测试使用自动滚动机制验证功能
**负责人**: David  
**完成时间**: 2026-03-14 13:07  
**验证**: 并发测试通过 (go test -race -count=5)

---

### 4. 无审计日志 ✅ 已完成
**来源**: 安全评估  
**风险**: 安全事件无法追溯 (抵赖风险)  
**修复方案**: 实现安全事件日志系统  
**负责人**: David  
**完成时间**: 2026-03-14 13:10  

**实现详情**:
- 创建 `internal/audit/` 目录
- 实现 `internal/audit/events.go` - 事件类型定义
  - 定义所有审计事件类型 (认证成功/失败、限流违规、权限变更、配置修改等)
  - 定义事件严重性级别 (INFO, WARNING, ERROR, CRITICAL)
  - 实现 AuditEvent 结构体 (JSON 格式，包含时间戳、客户端 IP、用户 ID 等)
  - 提供流式 API 构建审计事件
  
- 实现 `internal/audit/logger.go` - 审计日志核心
  - 异步写入 (channel + 后台 goroutine) 避免阻塞主流程
  - 日志轮转 (按大小/时间) 防止磁盘占用过大
  - 自动清理旧日志 (默认 30 天)
  - 敏感信息脱敏 (API Key、密码等)
  - 支持同步写入 (关键事件)
  - 上下文集成 (RequestID 追踪)
  - 便捷函数 (LogAuthSuccess, LogAuthFailure, LogRateLimitViolation 等)
  
- 实现 `internal/grpc/audit_interceptor.go` - gRPC 拦截器集成
  - AuditInterceptor - 记录所有 RPC 调用
  - AuditAuthInterceptor - 包装认证拦截器，记录认证成功/失败
  - AuditRateLimitInterceptor - 包装限流拦截器，记录限流违规
  - 自动提取客户端 IP、用户 ID、请求 ID
  
- 更新 `internal/grpc/server.go` - 服务器集成
  - 添加审计日志配置选项 (AuditLogDir, AuditMaxFileSize, AuditMaxAge)
  - 自动初始化审计日志器
  - 链式拦截器集成 (Audit -> Auth -> RateLimit)
  
- 实现 `internal/audit/logger_test.go` - 完整测试覆盖
  - 14 个测试用例，覆盖率 >90%
  - 测试异步写入、日志轮转、脱敏功能等
  - 所有测试通过 ✅

**功能特性**:
- ✅ 结构化日志 (JSON 格式)
- ✅ 包含时间戳、客户端 IP、用户 ID、事件类型、结果
- ✅ 支持异步写入避免阻塞
- ✅ 支持日志轮转 (按大小/时间)
- ✅ 敏感信息脱敏 (API Key、密码、Token 等)
- ✅ 支持 RequestID 追踪
- ✅ 支持上下文传递
- ✅ 自动清理旧日志

**验证**:
```bash
$ go test ./internal/audit/... -v
=== RUN   TestNewAuditEvent
--- PASS: TestNewAuditEvent (0.00s)
# ... 14 个测试全部通过
PASS
ok  	github.com/wangminggit/distributed-bloom-filter/internal/audit    0.629s

$ go build ./internal/grpc/...
# 编译成功，无错误
```

---

### 5. 节点间通信未加密 ✅ 已完成
**来源**: 安全评估  
**风险**: 中间人攻击  
**修复方案**: Raft 节点间通信启用 TLS  
**负责人**: David  
**预计时间**: 2 小时  
**状态**: ✅ 已完成  
**完成时间**: 2026-03-14 13:20  

**实现细节**:
- 新增 `internal/raft/tls_transport.go` - TLS 流层实现 (150+ 行)
- 更新 `internal/raft/config.go` - 添加 TLSConfig 和 TLSRaftConfig 结构
- 更新 `internal/raft/node.go` - 集成 TLS 传输层 (createTLSTransport 方法)
- 创建 `configs/tls/raft-tls-config.yaml` - Raft TLS 配置文件
- 创建 `deploy/RAFT-TLS-GUIDE.md` - 完整配置和使用指南
- 创建 `.learnings/RAFT-TLS-IMPLEMENTATION.md` - 实现总结文档
- 新增 `internal/raft/tls_transport_test.go` - TLS 测试
- 启用 mTLS 双向认证 (RequireAndVerifyClientCert)
- 证书验证和主机名检查
- TLS 1.2 最低版本要求 (可配置 TLS 1.3)
- 默认启用 TLS (TLSEnabled: true)

**验证**:
- ✅ 代码编译通过 (go build ./internal/raft/...)
- ✅ 单元测试通过 (go test ./internal/raft/...)
- ✅ 证书生成验证通过 (./generate-certs.sh)
- ✅ 配置验证测试通过

**文档**:
- [deploy/RAFT-TLS-GUIDE.md](../deploy/RAFT-TLS-GUIDE.md) - 配置指南
- [.learnings/RAFT-TLS-IMPLEMENTATION.md](../.learnings/RAFT-TLS-IMPLEMENTATION.md) - 实现总结

---

## 📊 进度追踪

| 问题 | 负责人 | 状态 | 开始时间 | 完成时间 |
|------|--------|------|----------|----------|
| Raft FSM 数据竞争 | David | ✅ 已完成 | 2026-03-14 13:06 | 2026-03-14 13:10 |
| gRPC 拦截器覆盖 | David | ✅ 已完成 | 2026-03-14 13:05 | 2026-03-14 13:15 |
| WAL 死锁风险 | David | ✅ 已完成 | 2026-03-14 13:05 | 2026-03-14 13:07 |
| 无审计日志 | David | ✅ 已完成 | 2026-03-14 13:05 | 2026-03-14 13:10 |
| 节点间通信加密 | David | ✅ 已完成 | 2026-03-14 13:10 | 2026-03-14 13:20 |

---

## ✅ 验收标准

- [ ] 所有 P0 问题修复完成
- [ ] 修复后测试全部通过
- [ ] 无回归问题
- [ ] 代码审查通过
- [ ] 更新相关文档

---

---

## 🔧 WAL 死锁风险修复详情 (2026-03-14)

### 根本原因
原代码中存在 `rollFile()` 和 `rollFileLocked()` 两个方法：
- `rollFile()`: 公共方法，获取自己的锁后调用 `rollFileLocked()`
- `rollFileLocked()`: 内部方法，假设调用者已持有锁

**风险**: 如果未来有代码在已持有 `w.mu` 锁的情况下调用 `rollFile()`，会导致递归锁死锁。

### 修复方案
1. **移除 `rollFile()` 公共方法** - 消除死锁风险的根源
2. **重命名 `rollFileLocked()` → `doRollFile()`** - 使用 Go 惯用的内部方法命名
3. **添加锁约定文档** - 在 `WALWriter` 结构体上明确说明锁的使用规则
4. **更新测试** - 通过自动滚动机制验证功能，而非直接调用内部方法

### 代码变更
**文件**: `internal/wal/encryptor.go`

```go
// WALWriter WAL 写入器 (支持滚动)
// 
// 锁约定 (Lock Convention):
// - w.mu 保护所有可变状态 (currentFile, currentSize, currentIndex 等)
// - 所有公共方法 (Write, Close) 在入口处获取锁
// - doRollFile() 是内部方法，调用者必须已持有 w.mu 锁
// - 避免递归锁：doRollFile() 不会尝试获取 w.mu
type WALWriter struct {
    mu sync.Mutex
    // ...
}

// doRollFile 滚动文件（内部方法，调用者必须已持有 w.mu 锁）
// 锁约定：此方法假设调用者已经持有 w.mu 锁，不会重复获取
// 使用场景：仅在 Write() 和其他已持有锁的内部方法中调用
func (w *WALWriter) doRollFile() error {
    // 实现细节...
}
```

### 验证结果
```bash
$ go test -race -count=5 ./internal/wal/... -run "Concurrent"
ok      github.com/wangminggit/distributed-bloom-filter/internal/wal    1.047s

$ go test -v -race ./internal/wal/...
--- PASS: TestWALWriter_ConcurrentWrites (0.01s)
--- PASS: TestWALWriter_RollFileLocked (0.00s)
# ... 所有 33 个测试通过
PASS
ok      github.com/wangminggit/distributed-bloom-filter/internal/wal    1.045s
```

### 经验教训
- 内部方法命名应清晰表达锁约定（如 `doXxx` 表示需要调用者持锁）
- 避免暴露可能导致死锁的公共包装方法
- 在结构体上添加锁约定文档，帮助后续维护者理解

---

## 🔧 Raft FSM 数据竞争修复详情 (2026-03-14)

### 根本原因
原代码中存在两个独立的 `Apply` 实现:
1. **`Node.Apply()`** (`internal/raft/node.go:377`): Node 结构体直接实现 `raft.FSM` 接口
2. **`BloomFSM.Apply()`** (`internal/raft/fsm.go:82`): 独立的 FSM 实现，带有 WAL 集成

**风险**: 
- 生产代码中 `Node` 被传递给 `raft.NewRaft()` 作为 FSM
- 测试代码中直接使用 `BloomFSM`
- 两个 Apply 路径逻辑不完全一致，可能导致状态不一致
- 锁保护分散在两个地方，增加数据竞争风险

### 修复方案
1. **在 Node 中嵌入 BloomFSM** - 添加 `fsm *BloomFSM` 字段
2. **Node.Apply 委托给 BloomFSM** - 移除重复逻辑，确保单一路径
3. **统一 Snapshot/Restore** - 也委托给 BloomFSM 处理
4. **添加锁保护注释** - 明确说明 BloomFSM 内部的锁保护机制

### 代码变更

#### 文件 1: `internal/raft/node.go`

**添加 FSM 字段**:
```go
type Node struct {
    // ... 其他字段 ...
    
    // FSM - embedded state machine for Raft
    // All state changes go through this single FSM to ensure consistency
    fsm *BloomFSM
    
    // Runtime state
    mu sync.RWMutex
}
```

**初始化 FSM**:
```go
// Create the FSM - this is the single source of truth for state changes
fsm, err := NewBloomFSM(bloomFilter, walEncryptor, filepath.Join(config.DataDir, "wal"))
if err != nil {
    return nil, fmt.Errorf("failed to create FSM: %w", err)
}

node := &Node{
    // ...
    fsm: fsm,
}
```

**统一 Apply 方法**:
```go
// Apply applies a Raft log entry to the FSM (called by Raft on leader).
// This implements the raft.FSM interface.
//
// IMPORTANT: This is the ONLY path for FSM state changes.
// All state modifications must go through this method to ensure consistency.
// The actual state change logic is delegated to BloomFSM.Apply() to avoid
// duplicate implementations and potential data races.
func (n *Node) Apply(log *raft.Log) interface{} {
    // Delegate to the embedded FSM - this ensures a single unified Apply path
    result := n.fsm.Apply(log)

    // Update metadata service (non-critical, doesn't affect FSM state)
    if result == nil && n.metadataService != nil {
        var cmd Command
        if err := json.Unmarshal(log.Data, &cmd); err == nil {
            switch cmd.Type {
            case "add":
                n.metadataService.RecordAdd()
            case "remove":
                n.metadataService.RecordRemove()
            }
        }
    }

    // Update state manager
    n.stateManager.SetLastApplied(log.Index)

    return result
}
```

**统一 Snapshot/Restore**:
```go
func (n *Node) Snapshot() (raft.FSMSnapshot, error) {
    return n.fsm.Snapshot()
}

func (n *Node) Restore(rc io.ReadCloser) error {
    err := n.fsm.Restore(rc)
    if err != nil {
        return err
    }
    n.snapshotManager.RestoreFromFSM(n.fsm.GetLastAppliedIndex(), n.fsm.GetLastAppliedTerm())
    return nil
}
```

#### 文件 2: `internal/raft/fsm.go`

**增强 Apply 方法注释**:
```go
// Apply applies a Raft log entry to the FSM.
// This implements the raft.FSM interface.
//
// IMPORTANT: This is the single source of truth for FSM state changes.
// All state modifications MUST go through this method to ensure consistency.
func (f *BloomFSM) Apply(log *raft.Log) interface{} {
    f.mu.Lock()
    defer f.mu.Unlock()
    // ... 实现细节 ...
}
```

#### 文件 3: `internal/raft/snapshot.go`

**添加 RestoreFromFSM 方法**:
```go
// RestoreFromFSM updates snapshot manager state after FSM restore.
// This is called after Restore() to keep snapshot manager in sync with FSM.
func (sm *SnapshotManager) RestoreFromFSM(index, term uint64) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.lastSnapshotIndex = index
    sm.lastSnapshotTerm = term
    sm.lastSnapshotTime = time.Now()
}
```

#### 文件 4: `internal/raft/tls_transport.go`

**修复缺失的导入**:
```go
import (
    // ...
    "io"  // 添加缺失的导入
    // ...
)
```

### 验证结果
```bash
$ go test -race ./internal/raft/... -count=2
ok      github.com/wangminggit/distributed-bloom-filter/internal/raft    11.190s

$ go test -race ./internal/raft/... -v
=== RUN   TestNodeStartAndLeaderElection
--- PASS: TestNodeStartAndLeaderElection (2.02s)
=== RUN   TestNodeAddAndContains
--- PASS: TestNodeAddAndContains (1.04s)
# ... 所有 16 个测试通过
PASS
ok      github.com/wangminggit/distributed-bloom-filter/internal/raft    8.543s
```

**结果**: ✅ 所有测试通过，无数据竞争警告

### 架构改进
修复后的架构:
```
┌─────────────────────────────────────┐
│              Raft Node              │
│  (implements raft.FSM interface)    │
├─────────────────────────────────────┤
│  Apply(log) → delegates to FSM      │
│  Snapshot() → delegates to FSM      │
│  Restore(rc) → delegates to FSM     │
├─────────────────────────────────────┤
│  fsm: *BloomFSM ← Single source     │
│       of truth for state changes    │
└─────────────────────────────────────┘
           │
           ↓ applies to
┌─────────────────────────────────────┐
│           BloomFSM                  │
│  (embedded state machine)           │
├─────────────────────────────────────┤
│  - mu sync.RWMutex (lock protection)│
│  - bloom: *CountingBloomFilter      │
│  - wal: *WALEncryptor               │
│  - walWriter: *WALWriter            │
└─────────────────────────────────────┘
```

### 经验教训
- **单一事实来源 (Single Source of Truth)**: FSM 状态变更必须通过唯一路径
- **委托模式 (Delegation Pattern)**: 避免重复实现，使用组合而非重复代码
- **锁保护集中化**: 所有状态保护的锁应该在同一个地方 (BloomFSM 内部)
- **清晰的注释**: 在关键方法上注明"这是唯一的状态变更路径"

---

*Last updated: 2026-03-14 13:10*
