# 新增/建议测试用例清单

**创建时间**: 2026-03-13  
**负责人**: Sarah Liu  
**来源**: 测试覆盖率分析和审查报告

---

## 📋 优先级说明

- **P0**: 必须实现，影响安全性或核心功能
- **P1**: 应该实现，影响质量或边界处理
- **P2**: 建议实现，提高覆盖率或可维护性

---

## 🔴 P0 测试用例 (必须实现)

### 1. TLS 配置测试

**文件**: `internal/grpc/tls_test.go` (新建)

```go
// TestTLSConfiguration_LoadValidCert
// 目的：验证有效证书可以正确加载
// 优先级：P0
// 预计工作量：2 小时
func TestTLSConfiguration_LoadValidCert(t *testing.T)

// TestTLSConfiguration_InvalidCert
// 目的：验证无效证书被拒绝
// 优先级：P0
// 预计工作量：1 小时
func TestTLSConfiguration_InvalidCert(t *testing.T)

// TestTLSConfiguration_ExpiredCert
// 目的：验证过期证书被拒绝
// 优先级：P0
// 预计工作量：2 小时
func TestTLSConfiguration_ExpiredCert(t *testing.T)

// TestMTLSHandshake_Success
// 目的：验证 mTLS 双向认证成功场景
// 优先级：P0
// 预计工作量：3 小时
func TestMTLSHandshake_Success(t *testing.T)

// TestMTLSHandshake_ClientCertRejected
// 目的：验证客户端证书无效时连接被拒绝
// 优先级：P0
// 预计工作量：2 小时
func TestMTLSHandshake_ClientCertRejected(t *testing.T)

// TestTLSHandshake_PlainConnectionRejected
// 目的：验证 TLS 服务器拒绝明文连接
// 优先级：P0
// 预计工作量：2 小时
func TestTLSHandshake_PlainConnectionRejected(t *testing.T)
```

**小计**: 6 个测试用例，12 小时

---

### 2. Race Detection 修复相关测试

**文件**: `internal/grpc/server_test.go` (修改)

```go
// 修改 setupTestServer 使用 InmemStore 代替 BoltStore
// 目的：避免 bolt DB 的 checkptr 问题，使 race detection 可以通过
// 优先级：P0
// 预计工作量：2 小时
func setupTestServer(t *testing.T) (*DBFServer, func())

// TestServer_ConcurrentRequests
// 目的：验证并发请求处理正确 (race detection)
// 优先级：P0
// 预计工作量：3 小时
func TestServer_ConcurrentRequests(t *testing.T)
```

**小计**: 2 个测试用例，5 小时

---

### 3. 认证拦截器边界测试

**文件**: `internal/grpc/interceptors_test.go` (修改)

```go
// TestAuthInterceptor_BoundaryTimestamp
// 目的：验证刚好 5 分钟前的时间戳 (边界情况)
// 优先级：P0
// 预计工作量：1 小时
func TestAuthInterceptor_BoundaryTimestamp(t *testing.T)

// TestAuthInterceptor_JustExpiredTimestamp
// 目的：验证 5 分 1 秒前的时间戳 (刚好过期)
// 优先级：P0
// 预计工作量：1 小时
func TestAuthInterceptor_JustExpiredTimestamp(t *testing.T)

// TestAuthInterceptor_ReplayAttack
// 目的：验证重放攻击防护 (同一请求重复提交)
// 优先级：P0
// 预计工作量：2 小时
func TestAuthInterceptor_ReplayAttack(t *testing.T)

// TestAuthInterceptor_EmptyAPIKey
// 目的：验证空 API Key 被拒绝
// 优先级：P0
// 预计工作量：1 小时
func TestAuthInterceptor_EmptyAPIKey(t *testing.T)
```

**小计**: 4 个测试用例，5 小时

---

### 4. 限流拦截器恢复测试

**文件**: `internal/grpc/interceptors_test.go` (修改)

```go
// TestRateLimitInterceptor_TokenRecovery
// 目的：验证令牌桶恢复机制
// 优先级：P0
// 预计工作量：2 小时
func TestRateLimitInterceptor_TokenRecovery(t *testing.T)

// TestRateLimitInterceptor_ConcurrentRateLimit
// 目的：验证并发请求限流正确
// 优先级：P0
// 预计工作量：2 小时
func TestRateLimitInterceptor_ConcurrentRateLimit(t *testing.T)
```

**小计**: 2 个测试用例，4 小时

---

## 🟠 P1 测试用例 (应该实现)

### 5. internal/wal/ 并发和边界测试

**文件**: `internal/wal/encryptor_test.go` (修改)

```go
// TestEncryptor_WrongKey
// 目的：验证错误密钥解密失败
// 优先级：P1
// 预计工作量：1 小时
func TestEncryptor_WrongKey(t *testing.T)

// TestWALWriter_ConcurrentWrites
// 目的：验证并发写入正确
// 优先级：P1
// 预计工作量：2 小时
func TestWALWriter_ConcurrentWrites(t *testing.T)

// TestWALWriter_RollingBoundary
// 目的：验证文件滚动边界条件 (刚好达到阈值)
// 优先级：P1
// 预计工作量：2 小时
func TestWALWriter_RollingBoundary(t *testing.T)

// TestWALReader_CorruptedFile
// 目的：验证损坏文件处理
// 优先级：P1
// 预计工作量：2 小时
func TestWALReader_CorruptedFile(t *testing.T)

// TestWALReader_EmptyDirectory
// 目的：验证空目录读取
// 优先级：P1
// 预计工作量：1 小时
func TestWALReader_EmptyDirectory(t *testing.T)

// TestK8sSecretLoader_MissingFile
// 目的：验证缺失文件处理
// 优先级：P1
// 预计工作量：1 小时
func TestK8sSecretLoader_MissingFile(t *testing.T)
```

**小计**: 6 个测试用例，9 小时

---

### 6. pkg/bloom/ 边界测试补充

**文件**: `pkg/bloom/counting_test.go` (修改)

```go
// TestNewCountingBloomFilter_EdgeCases
// 目的：验证 m=0, k=0 等边界情况
// 优先级：P1
// 预计工作量：1 小时
func TestNewCountingBloomFilter_EdgeCases(t *testing.T)

// TestAdd_NilItem
// 目的：验证 nil item 处理
// 优先级：P1
// 预计工作量：1 小时
func TestAdd_NilItem(t *testing.T)

// TestRemove_NonExistentItem
// 目的：验证删除不存在元素
// 优先级：P1
// 预计工作量：1 小时
func TestRemove_NonExistentItem(t *testing.T)

// TestDeserialize_CorruptedData
// 目的：验证损坏数据反序列化
// 优先级：P1
// 预计工作量：2 小时
func TestDeserialize_CorruptedData(t *testing.T)

// TestConcurrency_MixedOperations
// 目的：验证并发混合操作 (Add/Contains/Remove)
// 优先级：P1
// 预计工作量：2 小时
func TestConcurrency_MixedOperations(t *testing.T)
```

**小计**: 5 个测试用例，7 小时

---

### 7. internal/grpc/ 服务器错误处理测试

**文件**: `internal/grpc/server_test.go` (修改)

```go
// TestServer_RaftFailure
// 目的：验证 Raft 失败时的错误处理
// 优先级：P1
// 预计工作量：3 小时
func TestServer_RaftFailure(t *testing.T)

// TestServer_StreamInterceptor
// 目的：验证流式拦截器
// 优先级：P1
// 预计工作量：2 小时
func TestServer_StreamInterceptor(t *testing.T)
```

**小计**: 2 个测试用例，5 小时

---

### 8. cmd/server/ 基础测试 (新建)

**文件**: `cmd/server/main_test.go` (新建)

```go
// TestMain_NormalStartup
// 目的：验证服务器正常启动
// 优先级：P1
// 预计工作量：2 小时
func TestMain_NormalStartup(t *testing.T)

// TestMain_ConfigLoadFailure
// 目的：验证配置加载失败处理
// 优先级：P1
// 预计工作量：2 小时
func TestMain_ConfigLoadFailure(t *testing.T)

// TestMain_PortInUse
// 目的：验证端口占用处理
// 优先级：P1
// 预计工作量：2 小时
func TestMain_PortInUse(t *testing.T)
```

**文件**: `cmd/server/config_test.go` (新建)

```go
// TestConfig_LoadValidConfig
// 目的：验证有效配置加载
// 优先级：P1
// 预计工作量：1 小时
func TestConfig_LoadValidConfig(t *testing.T)

// TestConfig_LoadInvalidConfig
// 目的：验证无效配置拒绝
// 优先级：P1
// 预计工作量：1 小时
func TestConfig_LoadInvalidConfig(t *testing.T)

// TestConfig_DefaultValues
// 目的：验证配置默认值
// 优先级：P1
// 预计工作量：1 小时
func TestConfig_DefaultValues(t *testing.T)

// TestConfig_SignalGracefulShutdown
// 目的：验证 SIGINT/SIGTERM 优雅关闭
// 优先级：P1
// 预计工作量：2 小时
func TestConfig_SignalGracefulShutdown(t *testing.T)
```

**小计**: 7 个测试用例，11 小时

---

### 9. internal/metadata/ 基础测试 (新建)

**文件**: `internal/metadata/service_test.go` (新建)

```go
// TestService_Get
// 目的：验证元数据获取
// 优先级：P1
// 预计工作量：1 小时
func TestService_Get(t *testing.T)

// TestService_Set
// 目的：验证元数据设置
// 优先级：P1
// 预计工作量：1 小时
func TestService_Set(t *testing.T)

// TestService_Delete
// 目的：验证元数据删除
// 优先级：P1
// 预计工作量：1 小时
func TestService_Delete(t *testing.T)

// TestService_ConcurrentAccess
// 目的：验证并发访问正确
// 优先级：P1
// 预计工作量：2 小时
func TestService_ConcurrentAccess(t *testing.T)

// TestService_Persistence
// 目的：验证元数据持久化
// 优先级：P1
// 预计工作量：2 小时
func TestService_Persistence(t *testing.T)

// TestService_Recovery
// 目的：验证元数据恢复
// 优先级：P1
// 预计工作量：2 小时
func TestService_Recovery(t *testing.T)
```

**小计**: 6 个测试用例，9 小时

---

## 🟡 P2 测试用例 (建议实现)

### 10. internal/wal/ 高级测试

**文件**: `internal/wal/encryptor_test.go` (修改)

```go
// TestWALWriter_DiskFull
// 目的：验证磁盘空间不足处理
// 优先级：P2
// 预计工作量：2 小时
func TestWALWriter_DiskFull(t *testing.T)

// TestEncryptor_MultipleRotations
// 目的：验证多次密钥轮换
// 优先级：P2
// 预计工作量：2 小时
func TestEncryptor_MultipleRotations(t *testing.T)

// TestWALWriter_LargeWrite
// 目的：验证大写入处理
// 优先级：P2
// 预计工作量：1 小时
func TestWALWriter_LargeWrite(t *testing.T)
```

**小计**: 3 个测试用例，5 小时

---

### 11. pkg/bloom/ 性能测试

**文件**: `pkg/bloom/counting_test.go` (修改)

```go
// TestBloomFilter_FalsePositiveRate
// 目的：验证假阳性率符合预期
// 优先级：P2
// 预计工作量：2 小时
func TestBloomFilter_FalsePositiveRate(t *testing.T)

// TestBloomFilter_Performance
// 目的：基准测试性能
// 优先级：P2
// 预计工作量：2 小时
func BenchmarkBloomFilter_Add(b *testing.B)
func BenchmarkBloomFilter_Contains(b *testing.B)
```

**小计**: 3 个测试用例，4 小时

---

### 12. internal/grpc/ 高级测试

**文件**: `internal/grpc/interceptors_test.go` (修改)

```go
// TestRateLimitInterceptor_Configurations
// 目的：表驱动测试不同限流配置
// 优先级：P2
// 预计工作量：2 小时
func TestRateLimitInterceptor_Configurations(t *testing.T)

// TestAuthInterceptor_DifferentMethods
// 目的：验证不同 gRPC 方法的认证
// 优先级：P2
// 预计工作量：1 小时
func TestAuthInterceptor_DifferentMethods(t *testing.T)
```

**文件**: `internal/grpc/server_test.go` (修改)

```go
// TestServer_BatchOperations_LargeBatch
// 目的：验证大批量操作
// 优先级：P2
// 预计工作量：2 小时
func TestServer_BatchOperations_LargeBatch(t *testing.T)

// TestServer_Remove_NonExistentItem
// 目的：验证删除不存在元素
// 优先级：P2
// 预计工作量：1 小时
func TestServer_Remove_NonExistentItem(t *testing.T)
```

**小计**: 4 个测试用例，6 小时

---

### 13. internal/raft/ 测试 (模块实现后)

**文件**: `internal/raft/node_test.go` (修复和补充)

```go
// 首先修复现有测试的构建错误
// 然后添加以下测试:

// TestNode_StartLeaderElection
// 目的：验证 Leader 选举
// 优先级：P2
// 预计工作量：3 小时
func TestNode_StartLeaderElection(t *testing.T)

// TestNode_StartFollowerJoin
// 目的：验证 Follower 加入
// 优先级：P2
// 预计工作量：3 小时
func TestNode_StartFollowerJoin(t *testing.T)

// TestNode_Apply_LogReplication
// 目的：验证日志复制
// 优先级：P2
// 预计工作量：3 小时
func TestNode_Apply_LogReplication(t *testing.T)

// TestNode_Apply_MajorityConfirm
// 目的：验证多数派确认
// 优先级：P2
// 预计工作量：3 小时
func TestNode_Apply_MajorityConfirm(t *testing.T)

// TestNode_Shutdown_Graceful
// 目的：验证优雅关闭
// 优先级：P2
// 预计工作量：2 小时
func TestNode_Shutdown_Graceful(t *testing.T)

// TestNode_NetworkPartition_Recovery
// 目的：验证网络分区恢复
// 优先级：P2
// 预计工作量：4 小时
func TestNode_NetworkPartition_Recovery(t *testing.T)

// TestNode_LeaderFailover
// 目的：验证 Leader 故障转移
// 优先级：P2
// 预计工作量：4 小时
func TestNode_LeaderFailover(t *testing.T)
```

**小计**: 7 个测试用例，22 小时

---

## 📊 汇总统计

### 按优先级

| 优先级 | 测试用例数 | 预计工作量 |
|--------|------------|------------|
| P0 | 14 | 26 小时 |
| P1 | 26 | 41 小时 |
| P2 | 17 | 37 小时 |
| **总计** | **57** | **104 小时** |

### 按模块

| 模块 | P0 | P1 | P2 | 总计 |
|------|----|----|----|------|
| internal/grpc/ | 12 | 4 | 6 | 22 |
| internal/wal/ | 0 | 6 | 3 | 9 |
| pkg/bloom/ | 0 | 5 | 3 | 8 |
| cmd/server/ | 0 | 7 | 0 | 7 |
| internal/metadata/ | 0 | 6 | 0 | 6 |
| internal/raft/ | 0 | 0 | 7 | 7 |
| **总计** | **12** | **28** | **19** | **59** |

---

## 📅 实施计划

### Week 1 (2026-03-13 ~ 2026-03-19)

**目标**: 完成所有 P0 测试用例

| 日期 | 任务 | 用例数 | 工时 |
|------|------|--------|------|
| 03-13 | TLS 配置测试 (6 个) | 6 | 12h |
| 03-14 | Race Detection 修复 (2 个) | 2 | 5h |
| 03-15 | 认证拦截器边界测试 (4 个) | 4 | 5h |
| 03-16 | 限流拦截器恢复测试 (2 个) | 2 | 4h |
| **小计** | | **14** | **26h** |

**里程碑**: P0 测试完成，覆盖率提升至 ~60%

---

### Week 2 (2026-03-20 ~ 2026-03-26)

**目标**: 完成 P1 测试用例 (50%)

| 日期 | 任务 | 用例数 | 工时 |
|------|------|--------|------|
| 03-20 | WAL 并发和边界测试 (6 个) | 6 | 9h |
| 03-21 | Bloom 边界测试补充 (5 个) | 5 | 7h |
| 03-22 | gRPC 服务器错误处理 (2 个) | 2 | 5h |
| 03-23 | cmd/server 基础测试 (4 个) | 4 | 7h |
| 03-24 | cmd/server 配置测试 (3 个) | 3 | 4h |
| 03-25 | internal/metadata 测试 (6 个) | 6 | 9h |
| **小计** | | **26** | **41h** |

**里程碑**: P1 测试完成，覆盖率提升至 ~75%

---

### Week 3 (2026-03-27 ~ 2026-04-02)

**目标**: 完成 P2 测试用例 (50%) + 性能测试

| 日期 | 任务 | 用例数 | 工时 |
|------|------|--------|------|
| 03-27 | WAL 高级测试 (3 个) | 3 | 5h |
| 03-28 | Bloom 性能测试 (3 个) | 3 | 4h |
| 03-29 | gRPC 高级测试 (4 个) | 4 | 6h |
| 03-30 ~ 04-02 | 性能测试执行 | - | 22h |
| **小计** | | **10** | **37h** |

**里程碑**: P2 测试完成，性能测试达标

---

### Week 4 (2026-04-03 ~ 2026-04-09)

**目标**: internal/raft/ 测试 + 最终验收

| 日期 | 任务 | 用例数 | 工时 |
|------|------|--------|------|
| 04-03 ~ 04-07 | Raft 测试 (7 个) | 7 | 22h |
| 04-08 | 最终覆盖率检查 | - | 4h |
| 04-09 | M5 验收 | - | 4h |
| **小计** | | **7** | **30h** |

**里程碑**: M5 测试完成，所有指标达标

---

## ✅ 验收标准

每个测试用例应满足:

- [ ] 测试名称清晰描述场景和预期结果
- [ ] 使用表驱动测试 (如适用)
- [ ] 断言明确 (使用 testify)
- [ ] 测试数据独立 (使用 t.TempDir())
- [ ] 测试可重复执行
- [ ] 测试运行时间 <10 秒 (单个测试)
- [ ] 通过 race detection (如适用)

---

## 📝 备注

1. **内部/raft/测试**: 依赖于模块实现完成，预计 Week 4 开始
2. **性能测试**: 需要独立测试环境，避免影响开发
3. **故障注入测试**: 单独在 tests/chaos/ 目录下执行

---

*文档维护：Sarah Liu*  
*最后更新：2026-03-13*
