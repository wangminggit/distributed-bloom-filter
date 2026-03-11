# DBF 测试目录

本目录包含 Distributed Bloom Filter (DBF) 项目的所有测试代码。

## 目录结构

```
tests/
├── integration/          # 集成测试
│   └── api_test.go      # gRPC API 集成测试
├── performance/          # 性能测试
│   └── benchmark_test.go # 基准测试和负载测试
├── chaos/               # 故障注入测试
│   └── failure_test.go  # 节点/网络故障测试
├── e2e/                 # 端到端测试
│   └── workflow_test.go # 完整工作流程测试
├── config.yaml          # 测试配置文件
├── Makefile             # 测试命令
└── go.mod               # Go 模块依赖
```

## 快速开始

### 运行所有测试

```bash
cd tests
make all
```

### 运行单元测试

```bash
make unit
```

### 运行集成测试

```bash
make integration
```

### 运行性能测试

```bash
make performance
```

### 运行故障测试

```bash
make chaos
```

### 生成覆盖率报告

```bash
make coverage
```

生成的报告：`coverage.html`

## 测试说明

### 集成测试 (`integration/`)

测试 gRPC API 的完整功能，包括：
- Add/Remove/Contains 操作
- 批量操作
- 并发操作

**前置条件**: gRPC API (M4) 完成

### 性能测试 (`performance/`)

验证系统性能指标：
- QPS 基准测试（目标：10 万 QPS）
- 延迟测试（P99 < 5ms, P95 < 3ms, Avg < 2ms）
- 负载稳定性测试

**前置条件**: 完整的 6 节点集群

### 故障测试 (`chaos/`)

验证系统高可用性：
- Leader 故障恢复（< 500ms）
- Follower 故障
- 网络分区
- Pod 恢复

**前置条件**: K8s 测试集群 + chaos-mesh

### 端到端测试 (`e2e/`)

完整场景测试：
- 完整工作流程
- 批量操作
- 集群扩缩容
- 数据一致性

**前置条件**: 完整的 K8s 集群

## 配置

编辑 `config.yaml` 自定义测试参数：

```yaml
performance:
  target_qps: 100000
  duration: 1h
  concurrent_clients: 100
  
chaos:
  leader_failure:
    recovery_timeout: 500ms
```

## 当前状态

- ✅ 测试计划制定完成
- ✅ 测试框架搭建完成
- ⏳ 集成测试开发中（等待 gRPC API）
- ⏳ 性能测试开发中
- ⏳ 故障测试开发中

## 下一步

1. **3/12**: 完成集成测试用例（至少 5 个）
2. **3/13**: 完成性能测试脚本
3. **3/14**: 完成故障测试
4. **3/15**: 执行测试并输出报告

## 联系

测试负责人：Sarah  
沟通渠道：Feishu
