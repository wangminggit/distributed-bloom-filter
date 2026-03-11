# Distributed Bloom Filter - Kubernetes 部署

## 快速部署

```bash
# 应用所有配置
kubectl apply -f deploy/k8s/

# 检查部署状态
kubectl get pods -l app=dbf
kubectl get svc -l app=dbf

# 查看日志
kubectl logs -l app=dbf-storage -f
```

## 架构说明

```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                    │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │              API Gateway (Deployment)               │ │
│  │  副本数：2-3 (HPA 自动扩缩容)                         │ │
│  │  资源：512Mi 内存，500m CPU                         │ │
│  └─────────────────────┬──────────────────────────────┘ │
│                        │                                  │
│              ┌─────────▼─────────┐                       │
│              │   LoadBalancer    │                       │
│              │   Service         │                       │
│              └─────────┬─────────┘                       │
│                        │                                  │
│  ┌─────────────────────┼──────────────────────────────┐ │
│  │         Storage (StatefulSet)                      │ │
│  │  副本数：6 (3 分片 × 2 副本 或 6 分片 × 1 副本)          │ │
│  │  资源：2Gi 内存，2 CPU, 10Gi 存储                   │ │
│  │                                                     │ │
│  │  dbf-storage-0 (Shard 0, Leader)                   │ │
│  │  dbf-storage-1 (Shard 0, Follower)                 │ │
│  │  dbf-storage-2 (Shard 1, Leader)                   │ │
│  │  dbf-storage-3 (Shard 1, Follower)                 │ │
│  │  dbf-storage-4 (Shard 2, Leader)                   │ │
│  │  dbf-storage-5 (Shard 2, Follower)                 │ │
│  └────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `namespace.yaml` | 创建命名空间 |
| `configmap.yaml` | 配置文件 ConfigMap |
| `statefulset.yaml` | Storage Node StatefulSet |
| `deployment.yaml` | API Gateway Deployment |
| `service.yaml` | Services (Headless + LoadBalancer) |
| `pvc.yaml` | 持久化卷声明 |
| `hpa.yaml` | 水平自动扩缩容 |
| `pdb.yaml` | Pod  disruptions 预算 |
| `monitoring.yaml` | ServiceMonitor (Prometheus) |

## 部署步骤

### 1. 创建命名空间

```bash
kubectl apply -f namespace.yaml
```

### 2. 配置存储类（如果需要）

确保集群有默认 StorageClass，或修改 `pvc.yaml` 中的 `storageClassName`。

### 3. 应用配置

```bash
kubectl apply -f configmap.yaml
kubectl apply -f pvc.yaml
kubectl apply -f statefulset.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

### 4. 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n dbf

# 检查 Service
kubectl get svc -n dbf

# 查看 Storage Node 日志
kubectl logs dbf-storage-0 -n dbf -f

# 查看 API Gateway 日志
kubectl logs deployment/dbf-gateway -n dbf -f
```

### 5. 测试连接

```bash
# 端口转发
kubectl port-forward svc/dbf-gateway 50051:50051 -n dbf

# 使用 grpcurl 测试
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"key":"test123"}' localhost:50051 dbf.DistributedBloomFilter/Add
```

## 扩缩容

### 水平扩容（增加分片）

```bash
# 修改 StatefulSet replicas
kubectl scale statefulset dbf-storage --replicas=9 -n dbf

# 新节点会自动加入一致性 hash 环
# 数据会后台迁移
```

### 垂直扩容（增加资源）

```bash
# 编辑 StatefulSet
kubectl edit statefulset dbf-storage -n dbf

# 修改 resources.limits.memory 和 resources.limits.cpu
```

### API Gateway 自动扩缩容

HPA 已配置，基于 CPU 使用率自动扩缩容：

```bash
kubectl get hpa -n dbf
```

## 故障恢复

### Pod 重启

K8s 会自动重启故障 Pod，数据从 PV 恢复。

### 节点永久故障

```bash
# 删除故障 Pod
kubectl delete pod dbf-storage-0 -n dbf

# K8s 会在其他节点重建 Pod
# 新 Pod 会从副本同步数据
```

### 数据恢复

```bash
# 查看快照
kubectl exec dbf-storage-0 -n dbf -- ls -la /data/snapshot/

# 查看 WAL
kubectl exec dbf-storage-0 -n dbf -- ls -la /data/wal/

# 手动触发快照
kubectl exec dbf-storage-0 -n dbf -- curl -X POST localhost:8080/admin/snapshot
```

## 监控

### Prometheus

如果集群有 Prometheus Operator，ServiceMonitor 会自动发现指标。

访问 Grafana，导入 `deploy/grafana/dashboard.json`。

### 关键指标

```promql
# QPS
rate(dbf_operations_total[1m])

# P99 延迟
histogram_quantile(0.99, rate(dbf_operation_duration_seconds_bucket[1m]))

# Leader 状态
dbf_raft_is_leader

# 元素数量
dbf_element_count

# 内存使用
dbf_memory_usage_bytes
```

## 备份

### 手动备份

```bash
# 备份快照
kubectl cp dbf-storage-0:/data/snapshot ./backup-snapshot

# 备份 WAL
kubectl cp dbf-storage-0:/data/wal ./backup-wal
```

### 自动备份

使用 CronJob 定期备份到对象存储：

```bash
kubectl apply -f backup-cronjob.yaml
```

## 升级

### 滚动升级

```bash
# 更新镜像
kubectl set image statefulset/dbf-storage dbf=yourorg/dbf:v0.2.0 -n dbf

# 观察升级进度
kubectl rollout status statefulset/dbf-storage -n dbf
```

### 回滚

```bash
kubectl rollout undo statefulset/dbf-storage -n dbf
```

## 卸载

```bash
# 删除所有资源
kubectl delete -f deploy/k8s/

# 删除 PV（谨慎！会丢失数据）
kubectl delete pvc -l app=dbf-storage -n dbf
```

## 生产环境建议

1. **资源限制**: 根据实际负载调整 resources.limits
2. **亲和性**: 配置 Pod 反亲和性，避免同一分片的副本在同一节点
3. **PDB**: 配置 PodDisruptionBudget，保证升级时仍有足够副本
4. **监控告警**: 配置关键指标告警（延迟、错误率、磁盘使用）
5. **备份策略**: 定期备份到对象存储（S3/GCS）
6. **网络策略**: 配置 NetworkPolicy，限制访问
7. **安全**: 启用 TLS，配置 RBAC

## 故障排查

### Pod 无法启动

```bash
# 查看事件
kubectl describe pod dbf-storage-0 -n dbf

# 查看日志
kubectl logs dbf-storage-0 -n dbf --previous
```

### 数据不一致

```bash
# 检查 Raft 状态
kubectl exec dbf-storage-0 -n dbf -- curl localhost:8080/admin/raft/status

# 检查分片信息
kubectl exec dbf-storage-0 -n dbf -- curl localhost:8080/admin/shards
```

### 性能问题

```bash
# 查看 pprof
kubectl port-forward dbf-storage-0 6060:6060 -n dbf
# 浏览器访问 http://localhost:6060/debug/pprof/
```

## 常见问题

### Q: 如何确定分片数和副本数？

A: 根据数据量和 QPS 计算：
- 单分片容量：~2 亿元素
- 单分片内存：~120MB
- 单节点 QPS：~2 万
- 10 亿数据，10 万 QPS → 6 节点（3 分片 × 2 副本）

### Q: 数据会丢失吗？

A: 不会。WAL + 快照保证持久化，3 副本保证高可用。即使整个分片故障，也能从副本恢复。

### Q: 如何迁移数据？

A: 使用一致性 hash，新增节点时自动迁移相邻节点的部分数据。后台渐进式迁移，不影响前台服务。

---

详细文档见 [ARCHITECTURE.md](../../ARCHITECTURE.md)
