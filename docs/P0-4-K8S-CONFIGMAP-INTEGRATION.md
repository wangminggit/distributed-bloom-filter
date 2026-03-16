# P0-4 Raft 集成 - K8s ConfigMap 方案

## 概述

本文档描述如何在 Kubernetes 环境中集成 Raft 共识算法，使用 ConfigMap 管理集群配置。

## 端口配置

根据评审要求，端口分配如下：

| 服务 | 端口 | 协议 | 用途 |
|------|------|------|------|
| gRPC | 18080 | TCP | 客户端请求 |
| Raft | 18081 | TCP | Raft 共识通信 |
| Gossip | 18090 | TCP/UDP | memberlist 成员发现 |

### 端口说明

```yaml
# 端口分配原则
# - 基础端口：18080
# - Raft 端口：base + 1 = 18081
# - Gossip 端口：base + 10 = 18090
```

## K8s ConfigMap 集成方案

### 1. ConfigMap 定义

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dbf-cluster-config
  namespace: dbf-system
data:
  # 集群配置
  CLUSTER_NAME: "dbf-cluster"
  CLUSTER_SIZE: "3"
  
  # 节点配置
  NODE_PREFIX: "dbf-node"
  NODE_ID_TEMPLATE: "dbf-node-${POD_ORDINAL}"
  
  # 端口配置
  GRPC_PORT: "18080"
  RAFT_PORT: "18081"
  GOSSIP_PORT: "18090"
  
  # Raft 配置
  RAFT_ELECTION_TIMEOUT_MIN_MS: "150"
  RAFT_ELECTION_TIMEOUT_MAX_MS: "300"
  RAFT_HEARTBEAT_INTERVAL_MS: "50"
  
  # 成员列表配置
  MEMBERLIST_BIND_PORT: "18090"
  MEMBERLIST_ADVERTISE_PORT: "18090"
  
  # 初始集群种子节点（用于首次启动）
  INITIAL_PEERS: "dbf-node-0.dbf-headless.dbf-system.svc.cluster.local:18090"
```

### 2. Headless Service（用于 Pod DNS）

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dbf-headless
  namespace: dbf-system
spec:
  clusterIP: None  # Headless
  selector:
    app: dbf-node
  ports:
    - name: grpc
      port: 18080
      targetPort: grpc
    - name: raft
      port: 18081
      targetPort: raft
    - name: gossip
      port: 18090
      targetPort: gossip
      protocol: TCP
    - name: gossip-udp
      port: 18090
      targetPort: gossip
      protocol: UDP
```

### 3. StatefulSet 配置

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dbf-node
  namespace: dbf-system
spec:
  serviceName: "dbf-headless"
  replicas: 3
  selector:
    matchLabels:
      app: dbf-node
  template:
    metadata:
      labels:
        app: dbf-node
    spec:
      containers:
        - name: dbf-server
          image: dbf-server:latest
          command:
            - "/bin/dbf-server"
          args:
            - "--node-id=$(NODE_ID)"
            - "--port=$(GRPC_PORT)"
            - "--raft-port=$(RAFT_PORT)"
            - "--gossip-port=$(GOSSIP_PORT)"
            - "--bootstrap=$(BOOTSTRAP)"
          env:
            # 从 ConfigMap 加载配置
            - name: NODE_ID
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: POD_ORDINAL
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: GRPC_PORT
              valueFrom:
                configMapKeyRef:
                  name: dbf-cluster-config
                  key: GRPC_PORT
            - name: RAFT_PORT
              valueFrom:
                configMapKeyRef:
                  name: dbf-cluster-config
                  key: RAFT_PORT
            - name: GOSSIP_PORT
              valueFrom:
                configMapKeyRef:
                  name: dbf-cluster-config
                  key: GOSSIP_PORT
            - name: BOOTSTRAP
              value: "$(POD_ORDINAL == dbf-node-0)"
          ports:
            - name: grpc
              containerPort: 18080
              protocol: TCP
            - name: raft
              containerPort: 18081
              protocol: TCP
            - name: gossip
              containerPort: 18090
              protocol: TCP
            - name: gossip-udp
              containerPort: 18090
              protocol: UDP
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: dbf-data
  volumeClaimTemplates:
    - metadata:
        name: dbf-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### 4. 动态集群发现

#### 方案 A: 使用 DNS 发现

```go
// 从 K8s DNS 解析其他节点
func discoverPeersFromDNS(serviceName, namespace string, gossipPort int) ([]string, error) {
    // 查询 SRV 记录或 A 记录
    // 返回：["dbf-node-0.dbf-headless.dbf-system:18090", ...]
}
```

#### 方案 B: 使用 K8s API 发现

```go
// 通过 K8s API 获取 Pod 列表
func discoverPeersFromK8sAPI(namespace, labelSelector string) ([]string, error) {
    // 使用 clientset.CoreV1().Pods(namespace).List(...)
    // 返回所有匹配 Pod 的 IP:Port
}
```

### 5. 环境变量参考

| 变量名 | 来源 | 说明 |
|--------|------|------|
| `NODE_ID` | fieldRef (metadata.name) | 节点唯一标识 |
| `POD_ORDINAL` | fieldRef (metadata.name) | Pod 序号 |
| `GRPC_PORT` | ConfigMap | gRPC 监听端口 |
| `RAFT_PORT` | ConfigMap | Raft 通信端口 |
| `GOSSIP_PORT` | ConfigMap | Gossip 协议端口 |
| `BOOTSTRAP` | 计算得出 | 是否为初始节点 |
| `INITIAL_PEERS` | ConfigMap | 初始集群种子 |

## 部署流程

### 1. 创建命名空间

```bash
kubectl create namespace dbf-system
```

### 2. 应用 ConfigMap

```bash
kubectl apply -f configmap.yaml -n dbf-system
```

### 3. 应用 Headless Service

```bash
kubectl apply -f service-headless.yaml -n dbf-system
```

### 4. 应用 StatefulSet

```bash
kubectl apply -f statefulset.yaml -n dbf-system
```

### 5. 验证部署

```bash
# 查看 Pod 状态
kubectl get pods -n dbf-system -l app=dbf-node

# 查看日志
kubectl logs dbf-node-0 -n dbf-system -f

# 检查集群状态
kubectl exec dbf-node-0 -n dbf-system -- ./dbf-client stats
```

## 扩缩容

### 扩容

```bash
# 扩展到 5 个节点
kubectl scale statefulset dbf-node --replicas=5 -n dbf-system

# 新节点会自动通过 Gossip 协议加入集群
```

### 缩容

```bash
# 缩减到 2 个节点
kubectl scale statefulset dbf-node --replicas=2 -n dbf-system

# 注意：确保 Raft 集群保持多数派（quorum）
```

## 故障恢复

### 节点故障

1. K8s 自动重启故障 Pod
2. Raft 自动重新选举 Leader
3. 新节点通过 WAL 和快照恢复状态

### 数据恢复

```bash
# 从快照恢复
kubectl exec dbf-node-0 -n dbf-system -- ./dbf-server --restore-from=/data/snapshot/latest

# 从 WAL 恢复
kubectl exec dbf-node-0 -n dbf-system -- ./dbf-server --replay-wal
```

## 监控指标

```yaml
# Prometheus ServiceMonitor
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: dbf-monitor
  namespace: dbf-system
spec:
  selector:
    matchLabels:
      app: dbf-node
  endpoints:
    - port: grpc
      path: /metrics
      interval: 15s
```

## 安全配置

### 网络策略

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: dbf-network-policy
  namespace: dbf-system
spec:
  podSelector:
    matchLabels:
      app: dbf-node
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: dbf-node
      ports:
        - protocol: TCP
          port: 18080
        - protocol: TCP
          port: 18081
        - protocol: TCP
          port: 18090
        - protocol: UDP
          port: 18090
```

## 相关文件

- `deploy/k8s/configmap.yaml` - ConfigMap 定义
- `deploy/k8s/service-headless.yaml` - Headless Service
- `deploy/k8s/statefulset.yaml` - StatefulSet 定义
- `deploy/k8s/networkpolicy.yaml` - 网络策略
- `deploy/k8s/servicemonitor.yaml` - Prometheus 监控
