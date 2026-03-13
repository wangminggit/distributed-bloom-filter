# 部署指南

**版本**: v1.0.0-rc1  
**最后更新**: 2026-03-13

---

## 📋 目录

1. [部署方式](#部署方式)
2. [单机部署](#单机部署)
3. [Docker 部署](#docker-部署)
4. [Kubernetes 集群部署](#kubernetes-集群部署)
5. [配置说明](#配置说明)
6. [运维管理](#运维管理)
7. [故障排查](#故障排查)

---

## 部署方式

| 方式 | 适用场景 | 复杂度 |
|------|----------|--------|
| 单机模式 | 开发/测试 | ⭐ |
| Docker 容器 | 小规模生产 | ⭐⭐ |
| Kubernetes 集群 | 大规模生产 | ⭐⭐⭐⭐ |

---

## 单机部署

### 前置要求

- Go 1.21+
- Linux/macOS/Windows
- 至少 2GB 内存
- 至少 10GB 磁盘空间

### 安装

```bash
# 克隆源码
git clone https://github.com/wangminggit/distributed-bloom-filter.git
cd distributed-bloom-filter

# 编译
go build -o dbf ./cmd/server

# 验证
./dbf --version
```

### 启动

```bash
# 单机模式
./dbf server --mode=standalone --port=7000

# 或使用配置文件
./dbf server --config=config.yaml
```

### 验证

```bash
# 健康检查
curl http://localhost:8080/api/v1/status

# 添加元素
curl -X POST http://localhost:8080/api/v1/add \
  -H "Content-Type: application/json" \
  -d '{"key": "test-key"}'

# 查询元素
curl -X POST http://localhost:8080/api/v1/contains \
  -H "Content-Type: application/json" \
  -d '{"key": "test-key"}'
```

---

## Docker 部署

### 构建镜像

```bash
# 方式 1: 使用 Dockerfile
docker build -t dbf:latest .

# 方式 2: 多阶段构建 (推荐)
docker build -f Dockerfile.multi-stage -t dbf:latest .
```

### 运行容器

```bash
docker run -d \
  --name dbf \
  -p 7000:7000 \
  -p 8080:8080 \
  -v /data/dbf:/data \
  -v $(pwd)/config.yaml:/etc/dbf/config.yaml \
  -e DBF_MODE=standalone \
  -e DBF_API_KEY=your-api-key \
  --restart=unless-stopped \
  dbf:latest
```

### Docker Compose

创建 `docker-compose.yaml`:

```yaml
version: '3.8'

services:
  dbf:
    image: dbf:latest
    container_name: dbf
    ports:
      - "7000:7000"
      - "8080:8080"
    volumes:
      - dbf-data:/data
      - ./config.yaml:/etc/dbf/config.yaml
    environment:
      - DBF_MODE=standalone
      - DBF_API_KEY=your-api-key
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/status"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  dbf-data:
```

启动:

```bash
docker-compose up -d
```

---

## Kubernetes 集群部署

### 前置要求

- Kubernetes 1.19+
- kubectl 已配置
- 至少 5 个节点 (推荐)
- 持久化存储 (PV/PVC)

### 5 节点集群部署

#### 1. 创建命名空间

```bash
kubectl create namespace dbf-system
```

#### 2. 创建 ConfigMap

```bash
kubectl apply -f deploy/k8s/configmap.yaml
```

#### 3. 创建 Secret (API Key 和 TLS 证书)

```bash
# API Key
kubectl create secret generic dbf-secrets \
  --from-literal=api-key=$(openssl rand -hex 32) \
  -n dbf-system

# TLS 证书 (生产环境)
kubectl create secret tls dbf-tls \
  --cert=/path/to/tls.crt \
  --key=/path/to/tls.key \
  -n dbf-system
```

#### 4. 创建 StatefulSet

```bash
kubectl apply -f deploy/k8s/statefulset.yaml
```

#### 5. 创建 Headless Service

```bash
kubectl apply -f deploy/k8s/service-headless.yaml
```

#### 6. 创建 Gateway Service

```bash
kubectl apply -f deploy/k8s/service-gateway.yaml
```

#### 7. 验证部署

```bash
# 查看 Pod 状态
kubectl get pods -n dbf-system -l app=dbf

# 查看 Leader
kubectl logs dbf-0 -n dbf-system | grep "became leader"

# 查看集群状态
kubectl port-forward svc/dbf-gateway 8080:8080 -n dbf-system &
curl http://localhost:8080/api/v1/cluster
```

### 完整部署脚本

```bash
#!/bin/bash

set -e

NAMESPACE="dbf-system"

echo "🚀 Deploying DBF Cluster..."

# 创建命名空间
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# 应用所有资源
kubectl apply -f deploy/k8s/ -n $NAMESPACE

# 等待 Pod 就绪
echo "⏳ Waiting for Pods to be ready..."
kubectl wait --for=condition=ready pod -l app=dbf -n $NAMESPACE --timeout=300s

# 显示状态
echo "✅ Deployment complete!"
kubectl get pods -n $NAMESPACE
kubectl get svc -n $NAMESPACE

# 获取访问地址
GATEWAY_IP=$(kubectl get svc dbf-gateway -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo ""
echo "📡 Gateway Address: http://${GATEWAY_IP}:8080"
echo "🔧 gRPC Address: ${GATEWAY_IP}:7000"
```

### 扩缩容

```bash
# 扩容到 7 节点
kubectl scale statefulset dbf --replicas=7 -n dbf-system

# 缩容到 3 节点
kubectl scale statefulset dbf --replicas=3 -n dbf-system
```

### 滚动更新

```bash
# 更新镜像
kubectl set image statefulset/dbf dbf=dbf:v1.1.0 -n dbf-system

# 查看更新状态
kubectl rollout status statefulset/dbf -n dbf-system

# 回滚
kubectl rollout undo statefulset/dbf -n dbf-system
```

---

## 配置说明

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `DBF_MODE` | standalone | 运行模式 (standalone/cluster) |
| `DBF_PORT` | 7000 | gRPC 端口 |
| `DBF_HTTP_PORT` | 8080 | HTTP 端口 |
| `DBF_API_KEY` | - | API 密钥 (必需) |
| `DBF_DATA_DIR` | /data | 数据目录 |
| `DBF_TLS_CERT` | - | TLS 证书路径 |
| `DBF_TLS_KEY` | - | TLS 密钥路径 |
| `DBF_LOG_LEVEL` | info | 日志级别 |

### 配置文件示例

```yaml
# config.yaml
server:
  mode: cluster
  port: 7000
  httpPort: 8080
  dataDir: /data
  
cluster:
  # Raft 集群配置
  nodes:
    - host: dbf-0.dbf.dbf-system.svc.cluster.local
      port: 7000
    - host: dbf-1.dbf.dbf-system.svc.cluster.local
      port: 7000
    - host: dbf-2.dbf.dbf-system.svc.cluster.local
      port: 7000
    - host: dbf-3.dbf.dbf-system.svc.cluster.local
      port: 7000
    - host: dbf-4.dbf.dbf-system.svc.cluster.local
      port: 7000
  
  raft:
    electionTimeout: 200ms
    heartbeatInterval: 50ms
    snapshotInterval: 10000
    snapshotThreshold: 8192

security:
  # API Key 认证
  apiKey: your-api-key
  
  # TLS 配置
  tls:
    enabled: true
    certFile: /certs/server.crt
    keyFile: /certs/server.key
    clientAuth: RequireAndVerifyClientCert  # mTLS
  
  # 限流配置
  rateLimit:
    enabled: true
    requestsPerSecond: 100
    burstSize: 200

bloom:
  # Bloom Filter 配置
  capacity: 1000000000      # 10 亿容量
  falsePositiveRate: 0.001  # 0.1% 误判率

persistence:
  # WAL 配置
  wal:
    enabled: true
    dir: /data/wal
    segmentSize: 64MB
    syncInterval: 1s
  
  # 快照配置
  snapshot:
    enabled: true
    dir: /data/snapshot
    interval: 5m
    retainCount: 3

logging:
  level: info
  format: json
  output: stdout
```

---

## 运维管理

### 健康检查

```bash
# HTTP 健康检查
curl http://localhost:8080/api/v1/status

# gRPC 健康检查
grpc_health_probe -addr=localhost:7000
```

### 日志查看

```bash
# Docker
docker logs -f dbf

# Kubernetes
kubectl logs -f dbf-0 -n dbf-system

# 查看错误日志
kubectl logs dbf-0 -n dbf-system | grep ERROR
```

### 指标监控

DBF 暴露 Prometheus 格式的指标:

```bash
# 访问指标端点
curl http://localhost:8080/metrics
```

**关键指标**:

```promql
# 操作 QPS
rate(dbf_operations_total[1m])

# P99 延迟
histogram_quantile(0.99, rate(dbf_operation_duration_seconds_bucket[1m]))

# 元素数量
dbf_element_count

# 内存使用
dbf_memory_usage_bytes

# Raft 状态
dbf_raft_state{state="leader"}

# WAL 大小
dbf_wal_size_bytes
```

### 备份恢复

```bash
# 备份快照
kubectl cp dbf-0:/data/snapshot ./backup/snapshot -n dbf-system

# 恢复快照
kubectl cp ./backup/snapshot dbf-0:/data/snapshot -n dbf-system
kubectl exec dbf-0 -n dbf-system -- dbf admin reload-snapshot
```

---

## 故障排查

### Pod 无法启动

```bash
# 查看 Pod 状态
kubectl describe pod dbf-0 -n dbf-system

# 查看日志
kubectl logs dbf-0 -n dbf-system

# 常见原因:
# 1. 配置错误 - 检查 ConfigMap
# 2. 存储问题 - 检查 PV/PVC 状态
# 3. 端口冲突 - 检查端口占用
```

### Leader 选举失败

```bash
# 查看所有节点日志
for i in 0 1 2 3 4; do
  echo "=== dbf-$i ==="
  kubectl logs dbf-$i -n dbf-system | tail -20
done

# 检查网络连通性
kubectl exec dbf-0 -n dbf-system -- ping dbf-1.dbf-headless.dbf-system.svc.cluster.local

# 常见原因:
# 1. 网络分区 - 检查 Service 配置
# 2. 时钟不同步 - 检查节点时间
# 3. 防火墙 - 检查端口 7000/7001
```

### 数据不一致

```bash
# 查看各节点元素数量
for i in 0 1 2 3 4; do
  kubectl exec dbf-$i -n dbf-system -- \
    curl -s http://localhost:8080/api/v1/status | jq .element_count
done

# 触发快照同步
kubectl exec dbf-0 -n dbf-system -- dbf admin trigger-snapshot

# 常见原因:
# 1. 网络延迟 - 等待复制完成
# 2. 节点故障 - 检查故障节点日志
# 3. 配置不一致 - 检查各节点配置
```

### 性能问题

```bash
# 查看慢查询日志
kubectl logs dbf-0 -n dbf-system | grep "slow query"

# 压测
wrk -t12 -c400 -d30s http://dbf-gateway.dbf-system.svc.cluster.local:8080/api/v1/contains

# 常见原因:
# 1. 内存不足 - 增加资源限制
# 2. 磁盘 IO 瓶颈 - 使用 SSD
# 3. 网络延迟 - 检查网络配置
```

---

## 生产环境建议

### 1. 资源限制

```yaml
resources:
  requests:
    cpu: "2"
    memory: "4Gi"
  limits:
    cpu: "4"
    memory: "8Gi"
```

### 2. 反亲和性

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - dbf
      topologyKey: kubernetes.io/hostname
```

### 3. 持久化存储

```yaml
volumeClaimTemplates:
- metadata:
    name: data
  spec:
    accessModes: ["ReadWriteOnce"]
    storageClassName: "ssd"
    resources:
      requests:
        storage: 100Gi
```

### 4. 监控告警

配置 Prometheus 告警规则:

```yaml
groups:
- name: dbf
  rules:
  - alert: DBFNodeDown
    expr: up{job="dbf"} == 0
    for: 5m
    annotations:
      summary: "DBF node is down"
  
  - alert: DBFHighLatency
    expr: histogram_quantile(0.99, rate(dbf_operation_duration_seconds_bucket[5m])) > 0.01
    for: 10m
    annotations:
      summary: "DBF P99 latency > 10ms"
  
  - alert: DBFLeaderMissing
    expr: dbf_raft_state{state="leader"} == 0
    for: 2m
    annotations:
      summary: "No DBF leader elected"
```

---

**部署问题反馈**: https://github.com/wangminggit/distributed-bloom-filter/issues

**详细配置**: [CONFIGURATION.md](CONFIGURATION.md)
