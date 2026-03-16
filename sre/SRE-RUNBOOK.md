# SRE 规范 - Distributed Bloom Filter

**版本**: v1.0  
**创建日期**: 2026-03-16  
**维护者**: SRE Team

---

## 📋 目录

1. [SLI/SLO 定义](#slislo-定义)
2. [错误预算](#错误预算)
3. [告警策略](#告警策略)
4. [运维手册](#运维手册)
5. [事件响应](#事件响应)
6. [容量规划](#容量规划)
7. [备份恢复](#备份恢复)

---

## SLI/SLO 定义

### 服务级别指标 (SLI)

| 指标 | 定义 | 测量方法 |
|------|------|----------|
| **可用性** | 成功响应的请求比例 | `sum(rate(grpc_server_handled_total{grpc_code="OK"}[5m])) / sum(rate(grpc_server_handled_total[5m]))` |
| **延迟 (P50)** | 50% 请求的响应时间 | `histogram_quantile(0.50, rate(grpc_server_handling_seconds_bucket[5m]))` |
| **延迟 (P99)** | 99% 请求的响应时间 | `histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket[5m]))` |
| **数据一致性** | Raft 集群中数据一致的节点比例 | `dbf_raft_consistent_nodes / dbf_raft_total_nodes` |
| **写入成功率** | 成功写入的请求比例 | `sum(rate(dbf_writes_total{status="success"}[5m])) / sum(rate(dbf_writes_total[5m]))` |

### 服务级别目标 (SLO)

| 指标 | 目标值 | 时间窗口 | 严重级别 |
|------|--------|----------|----------|
| **可用性** | ≥ 99.9% | 30 天 | 🔴 P0 |
| **延迟 P50** | ≤ 10ms | 5 分钟 | 🟡 P2 |
| **延迟 P99** | ≤ 50ms | 5 分钟 | 🟠 P1 |
| **数据一致性** | 100% | 实时 | 🔴 P0 |
| **写入成功率** | ≥ 99.99% | 1 小时 | 🟠 P1 |

### 错误预算

| 周期 | 可用性预算 | 消耗速率 |
|------|-----------|----------|
| 7 天 | 0.7% (约 1 小时) | ~0.1%/天 |
| 30 天 | 0.1% (约 43 分钟) | ~0.003%/天 |

**错误预算消耗策略**:
- 消耗 < 50%: 正常发布节奏
- 消耗 50-80%: 谨慎发布，加强监控
- 消耗 > 80%: 冻结发布，专注稳定性

---

## 告警策略

### 告警级别定义

| 级别 | 响应时间 | 通知渠道 | 示例 |
|------|----------|----------|------|
| **P0 - Critical** | 5 分钟 | 电话 + 短信 + IM | 服务不可用、数据丢失 |
| **P1 - High** | 15 分钟 | 短信 + IM | 性能严重下降、部分功能不可用 |
| **P2 - Medium** | 1 小时 | IM | 性能轻微下降、非核心功能异常 |
| **P3 - Low** | 4 小时 | IM/邮件 | 资源预警、可自愈问题 |

### 核心告警规则

#### P0 - 服务不可用

```yaml
# deploy/alertmanager/rules-critical.yaml
groups:
  - name: dbf-critical
    rules:
      - alert: DBFServiceDown
        expr: up{job="dbf"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "DBF 服务不可用"
          description: "实例 {{ $labels.instance }} 已宕机超过 1 分钟"
          
      - alert: DBFNoLeader
        expr: sum(dbf_raft_is_leader) == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Raft 集群无 Leader"
          description: "集群中没有 Leader 节点，无法处理写入请求"
          
      - alert: DBFDataInconsistency
        expr: dbf_raft_consistent_nodes < dbf_raft_total_nodes
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "数据不一致"
          description: "集群中有 {{ $value }} 个节点数据不一致"
```

#### P1 - 性能告警

```yaml
# deploy/alertmanager/rules-high.yaml
groups:
  - name: dbf-high
    rules:
      - alert: DBFHighLatencyP99
        expr: histogram_quantile(0.99, rate(grpc_server_handling_seconds_bucket[5m])) > 0.05
        for: 5m
        labels:
          severity: high
        annotations:
          summary: "P99 延迟过高"
          description: "P99 延迟 {{ $value }}s 超过阈值 50ms"
          
      - alert: DBFHighErrorRate
        expr: sum(rate(grpc_server_handled_total{grpc_code!="OK"}[5m])) / sum(rate(grpc_server_handled_total[5m])) > 0.001
        for: 5m
        labels:
          severity: high
        annotations:
          summary: "错误率过高"
          description: "错误率 {{ $value | humanizePercentage }} 超过阈值 0.1%"
          
      - alert: DBFWriteFailures
        expr: sum(rate(dbf_writes_total{status="error"}[5m])) > 10
        for: 5m
        labels:
          severity: high
        annotations:
          summary: "写入失败过多"
          description: "每秒 {{ $value }} 次写入失败"
```

#### P2 - 资源告警

```yaml
# deploy/alertmanager/rules-medium.yaml
groups:
  - name: dbf-medium
    rules:
      - alert: DBFHighMemory
        expr: container_memory_usage_bytes{container="dbf"} / container_spec_memory_limit_bytes{container="dbf"} > 0.85
        for: 10m
        labels:
          severity: medium
        annotations:
          summary: "内存使用率过高"
          description: "内存使用率 {{ $value | humanizePercentage }} 超过阈值 85%"
          
      - alert: DBFHighCPU
        expr: rate(container_cpu_usage_seconds_total{container="dbf"}[5m]) / container_spec_cpu_quota{container="dbf"} * 100000 > 0.8
        for: 10m
        labels:
          severity: medium
        annotations:
          summary: "CPU 使用率过高"
          description: "CPU 使用率 {{ $value | humanizePercentage }} 超过阈值 80%"
          
      - alert: DBFDiskSpaceLow
        expr: kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes < 0.2
        for: 15m
        labels:
          severity: medium
        annotations:
          summary: "磁盘空间不足"
          description: "磁盘剩余空间 {{ $value | humanizePercentage }}"
```

#### P3 - 预警

```yaml
# deploy/alertmanager/rules-low.yaml
groups:
  - name: dbf-low
    rules:
      - alert: DBFSnapshotOld
        expr: time() - dbf_last_snapshot_timestamp > 3600
        for: 30m
        labels:
          severity: low
        annotations:
          summary: "快照过期"
          description: "上次快照时间超过 1 小时"
          
      - alert: DBFNodeNotReady
        expr: kube_pod_status_ready{pod=~"dbf-storage-.*", condition="true"} == 0
        for: 5m
        labels:
          severity: low
        annotations:
          summary: "节点未就绪"
          description: "Pod {{ $labels.pod }} 未就绪"
```

---

## 运维手册

### 日常检查清单

#### 每日检查

- [ ] 检查告警面板是否有未处理告警
- [ ] 查看 SLO 消耗情况
- [ ] 检查集群健康状态
- [ ] 查看错误日志趋势
- [ ] 确认备份完成

```bash
# 快速健康检查脚本
#!/bin/bash
# scripts/daily-check.sh

echo "=== DBF Daily Health Check ==="
echo ""

# 检查 Pod 状态
echo "📦 Pod Status:"
kubectl get pods -n dbf -l app=distributed-bloom-filter

# 检查 Leader 状态
echo ""
echo "👑 Leader Status:"
kubectl exec dbf-storage-0 -n dbf -- curl -s localhost:9090/metrics | grep dbf_raft_is_leader

# 检查错误率
echo ""
echo "⚠️  Error Rate (last 5m):"
curl -s 'http://prometheus:9090/api/v1/query?query=sum(rate(grpc_server_handled_total{grpc_code!="OK"}[5m]))/sum(rate(grpc_server_handled_total[5m]))' | jq '.data.result[0].value[1]'

# 检查 SLO
echo ""
echo "📊 Availability SLO (30d):"
curl -s 'http://prometheus:9090/api/v1/query?query=avg_over_time(up{job="dbf"}[30d])' | jq '.data.result[0].value[1]'
```

#### 每周检查

- [ ] 审查告警历史，优化误报
- [ ] 检查容量趋势
- [ ] 审查变更日志
- [ ] 更新运维文档
- [ ] 进行故障演练

### 标准操作程序 (SOP)

#### 节点扩容

```bash
# 1. 评估当前容量
kubectl top pods -n dbf
kubectl get hpa -n dbf

# 2. 调整 StatefulSet replicas
kubectl scale statefulset dbf-storage --replicas=9 -n dbf

# 3. 观察新节点加入
kubectl get pods -n dbf -w

# 4. 验证数据平衡
curl http://dbf-gateway.dbf:9090/metrics | grep dbf_shard_size
```

#### 版本升级

```bash
# 1. 备份当前状态
./scripts/backup.sh

# 2. 更新镜像
kubectl set image statefulset/dbf-storage dbf=yourorg/dbf:v0.2.0 -n dbf

# 3. 观察滚动升级
kubectl rollout status statefulset/dbf-storage -n dbf

# 4. 验证功能
./scripts/smoke-test.sh

# 5. 回滚方案 (如有问题)
kubectl rollout undo statefulset/dbf-storage -n dbf
```

#### 故障切换

```bash
# 1. 确认当前 Leader
kubectl exec dbf-storage-0 -n dbf -- curl localhost:9090/metrics | grep dbf_raft_is_leader

# 2. 强制 Leader 转移 (如需要)
kubectl exec dbf-storage-0 -n dbf -- curl -X POST localhost:9090/admin/raft/step-down

# 3. 验证新 Leader
kubectl get pods -n dbf -l app=distributed-bloom-filter -o wide
```

---

## 事件响应

### 事件分级

| 级别 | 定义 | 响应 |
|------|------|------|
| **SEV1** | 服务完全不可用，数据丢失 | 全员响应，War Room |
| **SEV2** | 核心功能受损，性能严重下降 | 值班工程师 + 备份 |
| **SEV3** | 非核心功能异常 | 值班工程师 |
| **SEV4** | 轻微问题，有 workaround | 工单处理 |

### 响应流程

```
1. 检测 → 2. 响应 → 3. 缓解 → 4. 修复 → 5. 复盘
```

### 事件模板

```markdown
## 事件报告

**事件 ID**: INC-YYYY-MM-DD-XXX
**级别**: SEV1/2/3/4
**状态**: 调查中/已缓解/已解决

### 时间线
- HH:MM - 检测到异常
- HH:MM - 开始响应
- HH:MM - 实施缓解措施
- HH:MM - 服务恢复
- HH:MM - 事件解决

### 影响范围
- 受影响服务:
- 受影响用户:
- 持续时间:

### 根本原因

### 修复措施

### 后续行动
- [ ] 行动项 1
- [ ] 行动项 2
```

---

## 容量规划

### 资源需求估算

| 组件 | CPU | 内存 | 存储 |
|------|-----|------|------|
| Storage Node | 2 核 | 2Gi | 10Gi |
| API Gateway | 1 核 | 512Mi | - |

### 容量公式

```
单节点容量 = Bloom Filter 大小 / 节点数
内存需求 = (计数器数量 × 4 bits) + 开销
存储需求 = WAL 大小 + 快照大小

示例 (10 亿元素，6 节点):
- 每节点元素：~1.67 亿
- 计数器数量：14.4 × 1.67 亿 ≈ 2.4 亿
- 内存：2.4 亿 × 4 bits ≈ 120 MB
- 总内存 (6 节点): ~720 MB
```

### 扩缩容策略

| 指标 | 扩容阈值 | 缩容阈值 |
|------|----------|----------|
| CPU 使用率 | > 70% | < 30% |
| 内存使用率 | > 80% | < 50% |
| QPS | > 8 万/节点 | < 2 万/节点 |
| 延迟 P99 | > 50ms | - |

---

## 备份恢复

### 备份策略

| 类型 | 频率 | 保留期 | 存储位置 |
|------|------|--------|----------|
| WAL 日志 | 实时 | 7 天 | 本地 + 对象存储 |
| 快照 | 每 5 分钟 | 30 天 | 本地 + 对象存储 |
| 全量备份 | 每天 | 90 天 | 对象存储 |

### 备份脚本

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/backup/dbf"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# 备份快照
kubectl exec dbf-storage-0 -n dbf -- tar czf /tmp/snapshot-$TIMESTAMP.tar.gz /data/snapshot
kubectl cp dbf-storage-0:/tmp/snapshot-$TIMESTAMP.tar.gz $BACKUP_DIR/

# 备份 WAL
kubectl exec dbf-storage-0 -n dbf -- tar czf /tmp/wal-$TIMESTAMP.tar.gz /data/wal
kubectl cp dbf-storage-0:/tmp/wal-$TIMESTAMP.tar.gz $BACKUP_DIR/

# 上传到对象存储
aws s3 cp $BACKUP_DIR/ s3://dbf-backups/$TIMESTAMP/ --recursive

# 清理本地备份
rm -rf $BACKUP_DIR/*
```

### 恢复流程

```bash
#!/bin/bash
# scripts/restore.sh

BACKUP_ID=$1

# 1. 停止服务
kubectl scale statefulset dbf-storage --replicas=0 -n dbf

# 2. 下载备份
aws s3 cp s3://dbf-backups/$BACKUP_ID/ /restore/ --recursive

# 3. 恢复数据
for i in 0 1 2 3 4 5; do
  kubectl cp /restore/snapshot-$BACKUP_ID.tar.gz dbf-storage-$i:/tmp/ -n dbf
  kubectl exec dbf-storage-$i -n dbf -- tar xzf /tmp/snapshot-$BACKUP_ID.tar.gz -C /
done

# 4. 重启服务
kubectl scale statefulset dbf-storage --replicas=6 -n dbf

# 5. 验证恢复
./scripts/verify-data.sh
```

---

## 附录

### 监控仪表板

- **运营仪表板**: 实时服务状态
- **SLO 仪表板**: 错误预算追踪
- **容量仪表板**: 资源使用趋势
- **业务仪表板**: QPS、延迟分布

### 联系人

| 角色 | 联系人 | 联系方式 |
|------|--------|----------|
| On-call | SRE 值班 | +86-XXX-XXXX |
| 负责人 | SRE Lead | email@example.com |
| 升级 | Engineering VP | vp@example.com |

### 相关文档

- [ARCHITECTURE.md](../ARCHITECTURE.md) - 架构设计
- [deploy/README.md](../deploy/README.md) - 部署指南
- [docs/P0-4-CHAOS-TEST-PLAN.md](./P0-4-CHAOS-TEST-PLAN.md) - 混沌测试

---

*最后更新：2026-03-16*
