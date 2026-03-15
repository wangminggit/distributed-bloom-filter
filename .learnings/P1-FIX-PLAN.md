# P1 问题修复计划 (2026-03-14)

**创建时间**: 2026-03-14 21:46  
**负责人**: Guawa (PM) + David (开发)  
**优先级**: 🟠 P1 (应该修复)  
**截止日期**: 2026-03-17

---

## 📋 P1 问题清单

### 来自代码评审 (5 项)

#### 1. Bloom Filter 反序列化缺少校验和验证 ✅
**来源**: 代码评审  
**风险**: 损坏数据无法检测  
**位置**: `pkg/bloom/counting.go::Deserialize()`  
**修复方案**: 添加 CRC32 校验和验证  
**负责人**: David  
**预计时间**: 1 小时  
**实际修复**:
- 导入 `hash/crc32` 包
- 新增 `ErrChecksumMismatch` 错误用于校验和验证失败
- 修改 `Serialize()`: 序列化格式变为 [m(4 字节)][k(4 字节)][counters(m 字节)][CRC32 校验和 (4 字节)]
- 修改 `Deserialize()`: 在返回前验证 CRC32 校验和，不匹配则返回 `ErrChecksumMismatch`
- 新增测试 `TestDeserialize_ChecksumVerification` 验证损坏数据检测
- 更新现有测试 `TestSerialize_EmptyFilter` 适配新的序列化格式

---

#### 2. Raft 节点关闭时 BoltDB 未显式关闭 ✅
**来源**: 代码评审  
**风险**: 资源泄漏  
**位置**: `internal/raft/node.go::Shutdown()`  
**修复方案**: 添加 `boltDB.Close()` 调用  
**负责人**: David  
**预计时间**: 0.5 小时  
**实际修复**:
- 在 `Shutdown()` 方法末尾添加 BoltDB 显式关闭逻辑
- 使用类型断言检查 `raftStore` 是否为 `*raftboltdb.BoltStore`
- 仅在使用持久化存储 (非 inmem) 时关闭
- 添加错误处理和关闭日志
- 关闭顺序：FSM → Transport → Raft → BoltDB

---

#### 3. Metadata 服务写入非原子，可能损坏
**来源**: 代码评审  
**风险**: 并发写入导致数据损坏  
**位置**: `internal/metadata/service.go`  
**修复方案**: 使用临时文件 + 原子重命名  
**负责人**: David  
**预计时间**: 1 小时

---

#### 4. gRPC Auth 拦截器内存泄漏风险
**来源**: 代码评审  
**风险**: 时间戳 map 无限增长  
**位置**: `internal/grpc/auth.go::cleanupOldTimestamps()`  
**修复方案**: 实现真正的清理逻辑 + 限制 map 大小  
**负责人**: David  
**预计时间**: 1 小时

---

#### 5. WAL 密钥轮换后旧密钥未持久化
**来源**: 代码评审  
**风险**: 重启后无法解密旧数据  
**位置**: `internal/wal/encryptor.go`  
**修复方案**: 持久化密钥历史到独立文件  
**负责人**: David  
**预计时间**: 2 小时

---

### 来自安全评估 (5 项)

#### 6. 流认证不完整
**来源**: 安全评估  
**风险**: 流式 RPC 可能被绕过  
**位置**: `internal/grpc/auth.go::StreamInterceptor()`  
**修复方案**: 实现完整的流认证逻辑  
**负责人**: David  
**预计时间**: 2 小时

---

#### 7. IP 欺骗漏洞 ✅
**来源**: 安全评估  
**风险**: X-Forwarded-For 可伪造，限流可绕过  
**位置**: `internal/grpc/ratelimit.go::GetClientIP()`  
**修复方案**: 仅信任可信代理的 X-Forwarded-For  
**负责人**: David  
**预计时间**: 1 小时  
**实际修复**:
- 添加 `TrustedProxies []string` 全局配置变量
- 实现 `initTrustedProxies()` 初始化可信代理网络列表
- 实现 `isTrustedProxy()` 检查 IP 是否在可信范围内
- 默认信任 localhost 和私有网络 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- 更新 `GetClientIP()` 仅当 peer IP 可信时才解析 X-Forwarded-For/X-Real-IP
- 更新 `getClientID()` 同样的安全逻辑
- 更新 `GetClientHTTP()` 同样的安全逻辑
- 新增配置文件 `configs/trusted-proxies.yaml`
- 新增测试 `TestGetClientIP_UntrustedProxy` 验证修复
- 新增测试 `TestIsTrustedProxy` 验证可信网络判断
- 新增测试 `TestGetClientIP_CustomTrustedProxies` 验证自定义配置
- 修复现有测试 `TestGetClientIP_WithMetadata` 添加 peer 信息

---

#### 8. 无密钥轮换机制
**来源**: 安全评估  
**风险**: 密钥长期不变增加泄露风险  
**位置**: `internal/wal/encryptor.go`  
**修复方案**: 实现定期密钥轮换 (7 天默认)  
**负责人**: David  
**预计时间**: 2 小时

---

#### 9. 无生产级密钥存储
**来源**: 安全评估  
**风险**: 密钥明文存储不安全  
**位置**: `configs/tls/`  
**修复方案**: 集成 KMS 或 HashiCorp Vault  
**负责人**: David  
**预计时间**: 3 小时

---

#### 10. 配置敏感信息可能泄露
**来源**: 安全评估  
**风险**: 日志/错误信息泄露密钥  
**位置**: 多处  
**修复方案**: 审查并脱敏所有敏感信息输出  
**负责人**: David  
**预计时间**: 1 小时

---

## 📊 进度追踪

| # | 问题 | 负责人 | 状态 | 开始时间 | 完成时间 |
|---|------|--------|------|----------|----------|
| 1 | Bloom 校验和验证 | David | ✅ 已完成 | 2026-03-14 21:48 | 2026-03-14 21:52 |
| 2 | Raft BoltDB 关闭 | David | ✅ 已完成 | 2026-03-14 21:48 | 2026-03-14 21:48 |
| 3 | Metadata 原子写入 | David | ✅ 已完成 | 2026-03-14 21:48 | 2026-03-14 21:50 |
| 4 | Auth 内存泄漏 | David | ✅ 已完成 | 2026-03-14 21:48 | 2026-03-14 22:15 |
| 5 | WAL 密钥持久化 | David | ⏳ 待开始 | - | - |
| 6 | 流认证完善 | David | ⏳ 待开始 | - | - |
| 7 | IP 欺骗修复 | David | ✅ 已完成 | 2026-03-14 21:50 | 2026-03-14 21:55 |
| 8 | 密钥轮换机制 | David | ⏳ 待开始 | - | - |
| 9 | 生产密钥存储 | David | ⏳ 待开始 | - | - |
| 10 | 敏感信息脱敏 | David | ⏳ 待开始 | - | - |

---

## ✅ 验收标准

- [ ] 所有 P1 问题修复完成
- [ ] 修复后测试全部通过
- [ ] 无回归问题
- [ ] 代码审查通过
- [ ] 更新相关文档

---

*Last updated: 2026-03-14 21:52*
