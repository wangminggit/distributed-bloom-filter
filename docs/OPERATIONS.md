# DBF 运维手册 (Operations Runbook)

**版本**: v0.1.0  
**最后更新**: 2026-03-17  
**适用环境**: 生产环境 / 预发布环境

---

## 📋 目录

1. [日常检查清单](#日常检查清单)
2. [服务启停](#服务启停)
3. [集群管理](#集群管理)
4. [备份与恢复](#备份与恢复)
5. [监控与告警](#监控与告警)
6. [性能调优](#性能调优)
7. [常见问题](#常见问题)

---

## 日常检查清单

### 每日检查

- [ ] **服务状态检查**
  ```bash
  # 检查进程是否运行
  ps aux | grep dbf-server
  
  # 检查端口监听
  netstat -tlnp | grep 8080
  ```

- [ ] **日志检查**
  ```bash
  # 查看最近的错误日志
  tail -100 /var/log/dbf/server.log | grep ERROR
  
  # 检查是否有异常重启
  grep "started\|stopped" /var/log/dbf/server.log | tail -20
  ```

- [ ] **集群健康度**
  ```bash
  # 使用 CLI 检查集群状态
  ./client --server localhost:8080 --action stats
  ```

- [ ] **磁盘空间**
  ```bash
  # 检查数据目录大小
  du -sh /var/lib/dbf/data/*
  
  # 检查可用空间
  df -h /var/lib/dbf
  ```

### 每周检查

- [ ] **WAL 文件清理** - 确认旧 WAL 文件已正确滚动
- [ ] **快照验证** - 检查最新快照完整性
- [ ] **证书有效期** - 检查 TLS 证书过期时间
- [ ] **性能指标回顾** - 检查延迟和吞吐量趋势

---

## 服务启停

### 启动服务

**Systemd 方式** (推荐):
```bash
sudo systemctl start dbf-server
sudo systemctl enable dbf-server  # 开机自启
```

**手动方式**:
```bash
# 作为 Leader 启动（集群第一个节点）
./server --bootstrap --node-id node1 --port 8080 --raft-port 8081

# 作为 Follower 启动（加入现有集群）
./server --node-id node2 --port 8080 --raft-port 8081
```

### 停止服务

**优雅停止** (推荐):
```bash
sudo systemctl stop dbf-server
# 或
kill -SIGTERM <pid>
```

**强制停止** (仅在紧急情况下):
```bash
kill -SIGKILL <pid>
```

⚠️ **注意**: 强制停止可能导致数据不一致，仅在优雅停止失败时使用。

### 重启服务

```bash
sudo systemctl restart dbf-server
```

重启后检查:
```bash
# 确认服务已启动
systemctl status dbf-server

# 查看启动日志
journalctl -u dbf-server -n 50
```

---

## 集群管理

### 查看集群状态

```bash
# 查看当前节点状态
./client --server localhost:8080 --action stats

# 输出示例:
# Server Statistics:
#   Node ID:      node1
#   Is Leader:    true
#   Raft State:   Leader
#   Leader:       node1
#   Bloom Size:   10000 bits
#   Bloom K:      3
#   Bloom Count:  ~1234 items
#   Raft Port:    8081
```

### 添加新节点

1. **准备新节点配置**
   ```bash
   # 在新服务器上
   mkdir -p /var/lib/dbf/data
   ```

2. **启动新节点**
   ```bash
   ./server --node-id node4 --port 8080 --raft-port 8081
   ```

3. **加入集群** (通过现有节点)
   ```bash
   # 在 Leader 节点上执行
   # TODO: 实现集群管理命令
   ```

4. **验证加入成功**
   ```bash
   ./client --server node4:8080 --action stats
   ```

### 移除节点

⚠️ **警告**: 移除节点前确保集群有足够副本（至少 3 节点）。

```bash
# 1. 停止目标节点
sudo systemctl stop dbf-server

# 2. 从集群配置中移除 (手动编辑或通过管理命令)
# 3. 重启其他节点使配置生效
```

### Leader 故障转移

Raft 会自动处理 Leader 故障转移：

1. Follower 检测到 Leader 心跳超时（默认 1 秒）
2. 触发新的选举
3. 获得多数票的节点成为新 Leader
4. 客户端重定向到新 Leader

**手动触发 Leader 转移** (维护场景):
```bash
# 1. 在当前 Leader 上停止服务
sudo systemctl stop dbf-server

# 2. 等待新 Leader 选举（约 1-2 秒）
# 3. 验证新 Leader
./client --server <follower-node>:8080 --action stats
```

---

## 备份与恢复

### 数据备份

**自动备份** (推荐):
```bash
# 添加 cron 任务
0 2 * * * /usr/local/bin/dbf-backup.sh
```

**手动备份**:
```bash
#!/bin/bash
# dbf-backup.sh

BACKUP_DIR="/backup/dbf"
DATA_DIR="/var/lib/dbf/data"
DATE=$(date +%Y%m%d_%H%M%S)

# 创建备份目录
mkdir -p $BACKUP_DIR

# 停止服务（确保数据一致性）
systemctl stop dbf-server

# 备份数据
tar -czf $BACKUP_DIR/dbf-backup-$DATE.tar.gz $DATA_DIR

# 启动服务
systemctl start dbf-server

# 清理旧备份（保留 7 天）
find $BACKUP_DIR -name "dbf-backup-*.tar.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR/dbf-backup-$DATE.tar.gz"
```

### 数据恢复

```bash
#!/bin/bash
# dbf-restore.sh

BACKUP_FILE=$1
DATA_DIR="/var/lib/dbf/data"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file>"
    exit 1
fi

# 停止服务
systemctl stop dbf-server

# 清空现有数据
rm -rf $DATA_DIR/*

# 恢复备份
tar -xzf $BACKUP_FILE -C /

# 启动服务
systemctl start dbf-server

echo "Restore completed from $BACKUP_FILE"
```

### 快照管理

```bash
# 查看快照文件
ls -lh /var/lib/dbf/data/raft/snapshots/

# 手动触发快照
# TODO: 实现管理命令

# 清理旧快照（保留最近 3 个）
ls -t /var/lib/dbf/data/raft/snapshots/*.snap | tail -n +4 | xargs rm
```

---

## 监控与告警

### Prometheus 指标

DBF 暴露以下指标（端口：`/metrics`）:

```
# Raft 相关
dbf_raft_is_leader
dbf_raft_term
dbf_raft_last_log_index
dbf_raft_last_log_term
dbf_raft_commit_index

# Bloom Filter 相关
dbf_bloom_size
dbf_bloom_k
dbf_bloom_element_count

# 操作统计
dbf_operations_total{type="add"}
dbf_operations_total{type="remove"}
dbf_operations_total{type="contains"}

# 性能指标
dbf_operation_duration_seconds{type="add",quantile="0.99"}
dbf_operation_duration_seconds{type="contains",quantile="0.99"}

# 资源使用
dbf_wal_size_bytes
dbf_snapshot_size_bytes
```

### Grafana 仪表板

导入 [DBF Dashboard](../deploy/grafana/dashboard.json) 获取预配置仪表板。

### 告警规则

```yaml
# prometheus/alerts.yml
groups:
- name: dbf
  rules:
  - alert: DBFNodeDown
    expr: up{job="dbf"} == 0
    for: 1m
    annotations:
      summary: "DBF node {{ $labels.instance }} is down"
  
  - alert: DBFNotLeader
    expr: dbf_raft_is_leader == 0
    for: 5m
    annotations:
      summary: "DBF node {{ $labels.instance }} is not leader for 5m"
  
  - alert: DBFHighLatency
    expr: histogram_quantile(0.99, rate(dbf_operation_duration_seconds_bucket[5m])) > 0.1
    for: 5m
    annotations:
      summary: "DBF p99 latency > 100ms"
  
  - alert: DBFWALGrowing
    expr: rate(dbf_wal_size_bytes[1h]) > 0
    for: 1h
    annotations:
      summary: "DBF WAL size continuously growing"
```

---

## 性能调优

### 关键参数

| 参数 | 默认值 | 说明 | 调优建议 |
|------|--------|------|----------|
| `--m` | 10000 | Bloom Filter 大小 (bits) | 根据预期元素数量调整 |
| `--k` | 3 | Hash 函数数量 | 通常 3-7，越多误判率越低 |
| `--raft-port` | 8081 | Raft 通信端口 | 确保防火墙开放 |
| `--data-dir` | ./data | 数据目录 | 使用 SSD 存储 |

### Bloom Filter 容量规划

```
预期元素数 × 9.6 bits/m ≈ 1% 误判率
预期元素数 × 14.4 bits/m ≈ 0.1% 误判率

示例:
100 万元素，1% 误判率: 100 万 × 9.6 = 9.6 Mbits ≈ 1.2 MB
```

### 性能优化建议

1. **使用 SSD 存储** - WAL 和快照频繁写入
2. **增加 Raft 心跳频率** - 更快故障检测
3. **批量操作** - 使用 `batch-add` 减少网络往返
4. **本地读** - Contains 操作可在任何节点执行

---

## 常见问题

### Q1: 节点无法启动

**症状**: `systemctl start dbf-server` 失败

**排查步骤**:
```bash
# 1. 查看日志
journalctl -u dbf-server -n 100

# 2. 检查端口占用
netstat -tlnp | grep 8080

# 3. 检查数据目录权限
ls -la /var/lib/dbf/data

# 4. 检查配置文件
cat /etc/dbf/config.yaml
```

**常见原因**:
- 端口被占用
- 数据目录权限错误
- 配置文件语法错误
- TLS 证书缺失或过期

### Q2: 集群无法选举 Leader

**症状**: 所有节点都是 Follower

**排查步骤**:
```bash
# 检查节点间网络连通性
ping <other-node>

# 检查 Raft 端口
telnet <other-node> 8081

# 查看 Raft 日志
grep "election\|vote" /var/log/dbf/server.log
```

**常见原因**:
- 网络分区
- 防火墙阻止 Raft 端口
- 节点 ID 冲突
- 时钟不同步

### Q3: 写入失败 "not the leader"

**症状**: `add` 操作返回错误

**解决方案**:
```bash
# 1. 找到当前 Leader
./client --server <any-node>:8080 --action stats

# 2. 向 Leader 发送请求
./client --server <leader>:8080 --action add --items "item1"
```

### Q4: 内存使用过高

**症状**: 进程内存持续增长

**排查步骤**:
```bash
# 检查 Bloom Filter 大小
./client --server localhost:8080 --action stats

# 检查 WAL 文件大小
du -sh /var/lib/dbf/data/wal/*

# 检查快照数量
ls -lh /var/lib/dbf/data/raft/snapshots/
```

**解决方案**:
- 定期清理旧 WAL 文件
- 限制快照数量
- 调整 Bloom Filter 大小

### Q5: 数据不一致

**症状**: 不同节点查询结果不同

**解决方案**:
```bash
# 1. 检查集群状态
./client --server <node>:8080 --action stats

# 2. 等待 Raft 同步（通常几秒内自动修复）

# 3. 如果持续不一致，重启 Follower 节点
sudo systemctl restart dbf-server
```

---

## 紧急联系人

| 角色 | 姓名 | 联系方式 |
|------|------|----------|
| On-call 工程师 | - | - |
| 技术负责人 | - | - |
| 产品负责人 | - | - |

---

## 附录

### A. 配置文件示例

```yaml
# /etc/dbf/config.yaml
server:
  port: 8080
  raft_port: 8081
  data_dir: /var/lib/dbf/data
  node_id: node1

bloom:
  size: 100000  # bits
  k: 5          # hash functions

security:
  enable_mtls: true
  ca_cert: /etc/dbf/certs/ca.crt
  server_cert: /etc/dbf/certs/server.crt
  server_key: /etc/dbf/certs/server.key

logging:
  level: info
  file: /var/log/dbf/server.log
```

### B. Systemd 服务配置

```ini
# /etc/systemd/system/dbf-server.service
[Unit]
Description=Distributed Bloom Filter Server
After=network.target

[Service]
Type=simple
User=dbf
Group=dbf
ExecStart=/usr/local/bin/dbf-server --config /etc/dbf/config.yaml
Restart=on-failure
RestartSec=5

# Security
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/dbf/data /var/log/dbf

[Install]
WantedBy=multi-user.target
```

---

*文档版本: v0.1.0 | 最后更新: 2026-03-17*
