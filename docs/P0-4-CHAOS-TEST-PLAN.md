# P0-4 Raft 混沌测试计划

## 概述

本测试计划涵盖 Raft 共识算法在分布式环境下的容错能力和一致性保证。包含 4 个核心混沌场景。

## 测试环境

### 集群配置

```yaml
节点数：3
副本数：3
Bloom Filter 配置:
  - m: 10000 bits
  - k: 3 hash functions
```

### 测试工具

- **Chaos Mesh** 或 **LitmusChaos** - 混沌工程平台
- **自定义测试脚本** - `tests/chaos/`
- **监控** - Prometheus + Grafana

---

## 混沌场景 1: Leader 节点故障

### 测试目标

验证 Leader 节点故障时，集群能够：
1. 检测到 Leader 失效
2. 选举新的 Leader
3. 恢复服务，不丢失已提交的数据

### 测试步骤

```bash
# 1. 启动 3 节点集群
./scripts/start-cluster.sh --nodes 3

# 2. 确认当前 Leader
./scripts/get-leader.sh
# 输出：Leader: node-1

# 3. 写入测试数据
./scripts/write-data.sh --count 100

# 4. 模拟 Leader 故障（kill 进程）
kubectl exec dbf-node-1 -n dbf-system -- kill -9 1

# 或者使用 Chaos Mesh
kubectl apply -f chaos-leader-failure.yaml
```

### 预期结果

| 指标 | 预期值 |
|------|--------|
| 选举完成时间 | < 500ms |
| 数据丢失 | 0 |
| 服务中断时间 | < 1s |
| 新 Leader 产生 | 是 |

### 验证脚本

```bash
# 等待选举完成
sleep 2

# 检查新 Leader
./scripts/get-leader.sh

# 验证数据完整性
./scripts/verify-data.sh --expected-count 100

# 检查集群健康
./scripts/cluster-health.sh
```

### 混沌实验配置 (Chaos Mesh)

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: leader-failure
  namespace: dbf-system
spec:
  action: pod-failure
  mode: one
  selector:
    labelSelectors:
      app: dbf-node
      statefulset.kubernetes.io/pod-name: dbf-node-1
  duration: 30s
```

---

## 混沌场景 2: 网络分区

### 测试目标

验证网络分区时，集群能够：
1. 检测分区
2. 保持分区内多数派可用
3. 分区恢复后自动同步数据

### 测试步骤

```bash
# 1. 启动 3 节点集群
./scripts/start-cluster.sh --nodes 3

# 2. 写入测试数据
./scripts/write-data.sh --count 200

# 3. 创建网络分区（隔离 node-2）
tc qdisc add dev eth0 root netem loss 100%
# 或使用 Chaos Mesh
kubectl apply -f chaos-network-partition.yaml

# 4. 在可用分区继续写入
./scripts/write-data.sh --count 50 --target node-0

# 5. 等待 30 秒后恢复网络
tc qdisc del dev eth0 root netem loss 100%
```

### 预期结果

| 指标 | 预期值 |
|------|--------|
| 分区检测时间 | < 200ms |
| 多数派服务可用 | 是 |
| 少数派拒绝写入 | 是 |
| 数据最终一致 | 是 |
| 数据丢失 | 0 |

### 验证脚本

```bash
# 恢复后等待同步
sleep 5

# 验证所有节点数据一致
./scripts/verify-consistency.sh --nodes 3

# 检查数据总数
./scripts/verify-data.sh --expected-count 250
```

### 混沌实验配置

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: network-partition
  namespace: dbf-system
spec:
  action: partition
  mode: one
  selector:
    labelSelectors:
      app: dbf-node
      statefulset.kubernetes.io/pod-name: dbf-node-2
  direction: both
  duration: 30s
```

---

## 混沌场景 3: 随机节点重启

### 测试目标

验证随机节点重启时，集群能够：
1. 保持服务可用
2. 重启节点自动重新加入集群
3. 通过 WAL 和快照恢复状态

### 测试步骤

```bash
# 1. 启动 3 节点集群
./scripts/start-cluster.sh --nodes 3

# 2. 持续写入数据
./scripts/continuous-write.sh --duration 60s --rate 10/s &

# 3. 随机重启节点（每 10 秒一个）
for i in 0 1 2; do
  kubectl delete pod dbf-node-$i -n dbf-system
  sleep 10
done

# 或使用 Chaos Mesh
kubectl apply -f chaos-pod-restart.yaml
```

### 预期结果

| 指标 | 预期值 |
|------|--------|
| 服务可用性 | > 99% |
| 重启节点重新加入 | 是 |
| 数据恢复完整 | 是 |
| 写入错误率 | < 1% |

### 验证脚本

```bash
# 等待所有节点恢复
sleep 30

# 检查集群状态
./scripts/cluster-health.sh

# 验证数据完整性
./scripts/verify-data.sh --tolerance 1%
```

### 混沌实验配置

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: pod-restart
  namespace: dbf-system
spec:
  action: pod-restart
  mode: random-max-percent
  selector:
    labelSelectors:
      app: dbf-node
  maxPercent: 34  # 每次重启 1 个节点（33%）
  duration: 60s
  scheduler:
    cron: "@every 10s"
```

---

## 混沌场景 4: 磁盘故障/数据损坏

### 测试目标

验证磁盘故障时，集群能够：
1. 检测磁盘写入失败
2. 切换到健康节点
3. 故障节点恢复后重建数据

### 测试步骤

```bash
# 1. 启动 3 节点集群
./scripts/start-cluster.sh --nodes 3

# 2. 写入测试数据
./scripts/write-data.sh --count 500

# 3. 模拟磁盘故障（只读或空间满）
kubectl exec dbf-node-1 -n dbf-system -- \
  mount -o remount,ro /data

# 或填满磁盘
kubectl exec dbf-node-1 -n dbf-system -- \
  dd if=/dev/zero of=/data/fill bs=1M count=1000

# 4. 尝试写入（应该失败并切换到其他节点）
./scripts/write-data.sh --count 100 --target node-1

# 5. 恢复磁盘
kubectl exec dbf-node-1 -n dbf-system -- \
  mount -o remount,rw /data
kubectl exec dbf-node-1 -n dbf-system -- \
  rm /data/fill
```

### 预期结果

| 指标 | 预期值 |
|------|--------|
| 磁盘故障检测 | < 100ms |
| 写入失败转移 | 是 |
| 数据不丢失 | 是 |
| 节点恢复后同步 | 是 |

### 验证脚本

```bash
# 恢复后等待同步
sleep 10

# 检查节点状态
./scripts/node-status.sh node-1

# 验证数据完整
./scripts/verify-data.sh --expected-count 600
```

### 混沌实验配置

```yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: IOChaos
metadata:
  name: io-failure
  namespace: dbf-system
spec:
  action: fault
  mode: one
  selector:
    labelSelectors:
      app: dbf-node
      statefulset.kubernetes.io/pod-name: dbf-node-1
  volumePath: /data
  percent: 100
  duration: 30s
```

---

## 测试报告模板

### 执行摘要

```
测试日期：YYYY-MM-DD
测试场景：[场景名称]
集群配置：3 节点
测试持续时间：X 分钟

结果：PASS / FAIL
```

### 详细指标

| 指标 | 预期 | 实际 | 状态 |
|------|------|------|------|
| 选举时间 | < 500ms | XXXms | ✅/❌ |
| 数据丢失 | 0 | X | ✅/❌ |
| 服务可用性 | > 99% | XX% | ✅/❌ |
| 恢复时间 | < 5s | Xs | ✅/❌ |

### 日志摘要

```
[时间戳] Leader 故障检测
[时间戳] 开始选举
[时间戳] 新 Leader 产生：node-X
[时间戳] 服务恢复
```

---

## 自动化测试脚本

### 运行所有混沌测试

```bash
#!/bin/bash
# tests/chaos/run-all.sh

set -e

echo "🚀 Starting Chaos Test Suite"

# 场景 1: Leader 故障
echo "📋 Scenario 1: Leader Failure"
./tests/chaos/leader-failure.sh

# 场景 2: 网络分区
echo "📋 Scenario 2: Network Partition"
./tests/chaos/network-partition.sh

# 场景 3: 随机重启
echo "📋 Scenario 3: Random Restart"
./tests/chaos/pod-restart.sh

# 场景 4: 磁盘故障
echo "📋 Scenario 4: Disk Failure"
./tests/chaos/disk-failure.sh

echo "✅ Chaos Test Suite Complete"
```

### 测试结果验证

```bash
#!/bin/bash
# tests/chaos/verify.sh

# 检查集群健康
kubectl get pods -n dbf-system -l app=dbf-node

# 检查 Leader 状态
./scripts/get-leader.sh

# 验证数据一致性
./scripts/verify-consistency.sh

# 检查监控指标
curl http://prometheus:9090/api/v1/query?query=dbf_raft_leader_changes
```

---

## 通过标准

所有混沌测试必须满足：

1. **零数据丢失** - 所有已提交的数据必须保留
2. **自动恢复** - 无需人工干预
3. **快速恢复** - 服务中断时间 < 5 秒
4. **一致性保证** - 最终所有节点数据一致

---

## 相关文件

- `tests/chaos/failure_test.go` - 混沌测试代码
- `tests/chaos/run-all.sh` - 测试执行脚本
- `deploy/chaos/` - Chaos Mesh 配置
- `docs/P0-4-K8S-CONFIGMAP-INTEGRATION.md` - K8s 集成方案
