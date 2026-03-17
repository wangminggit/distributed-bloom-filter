# DBF 故障排查指南 (Troubleshooting Guide)

**版本**: v0.1.0  
**最后更新**: 2026-03-17

---

## 📋 快速诊断流程

```
服务不可用？
├─ 检查进程状态 → systemctl status dbf-server
├─ 检查端口监听 → netstat -tlnp | grep 8080
├─ 查看最近日志 → journalctl -u dbf-server -n 50
└─ 检查网络连接 → ping/curl 测试

写入失败？
├─ 检查 Leader 状态 → ./client --action stats
├─ 查看 Raft 日志 → grep "election\|vote" server.log
└─ 验证集群连通性 → telnet <node> 8081

数据不一致？
├─ 等待 Raft 同步（通常 <5 秒）
├─ 检查节点健康状态
└─ 重启 Follower 节点

性能下降？
├─ 检查磁盘 I/O → iostat -x 1
├─ 检查内存使用 → free -h
├─ 查看慢查询日志
└─ 检查网络延迟
```

---

## 🔍 常见问题诊断

### 1. 服务启动失败

#### 症状
```bash
$ systemctl start dbf-server
Job for dbf-server.service failed because the control process exited with error code.
```

#### 诊断步骤

**步骤 1: 查看详细错误**
```bash
journalctl -u dbf-server -n 100 --no-pager
```

**步骤 2: 检查端口占用**
```bash
# 检查 8080 端口
netstat -tlnp | grep 8080
lsof -i :8080

# 检查 8081 端口 (Raft)
netstat -tlnp | grep 8081
```

**步骤 3: 检查数据目录**
```bash
# 检查目录存在性
ls -la /var/lib/dbf/data

# 检查权限
stat /var/lib/dbf/data

# 检查磁盘空间
df -h /var/lib/dbf
```

**步骤 4: 检查配置文件**
```bash
# 验证 YAML 语法
python3 -c "import yaml; yaml.safe_load(open('/etc/dbf/config.yaml'))"

# 检查文件内容
cat /etc/dbf/config.yaml
```

#### 常见原因及解决方案

| 错误信息 | 原因 | 解决方案 |
|---------|------|----------|
| `address already in use` | 端口被占用 | `kill` 占用进程或修改配置 |
| `permission denied` | 目录权限错误 | `chown dbf:dbf /var/lib/dbf` |
| `no space left on device` | 磁盘满 | 清理空间或扩容 |
| `failed to load certificate` | TLS 证书问题 | 检查证书路径和格式 |
| `invalid configuration` | 配置错误 | 修正配置文件 |

---

### 2. Raft 集群无法选举 Leader

#### 症状
- 所有节点状态都是 `Follower`
- 写入操作全部失败
- 日志中频繁出现选举超时

#### 诊断步骤

**步骤 1: 检查节点状态**
```bash
# 在每个节点上执行
./client --server localhost:8080 --action stats
```

**步骤 2: 查看选举日志**
```bash
grep -E "election|vote|candidate|leader" /var/log/dbf/server.log | tail -50
```

**步骤 3: 检查网络连通性**
```bash
# 节点间 ping 测试
ping <node2-ip>
ping <node3-ip>

# Raft 端口连通性
telnet <node2-ip> 8081
nc -zv <node2-ip> 8081
```

**步骤 4: 检查防火墙**
```bash
# 查看防火墙规则
iptables -L -n | grep 8081
firewall-cmd --list-all

# 临时关闭防火墙测试
systemctl stop firewalld
```

**步骤 5: 检查时钟同步**
```bash
# 检查时间
date
timedatectl status

# 检查 NTP 同步
chronyc tracking
```

#### 常见原因及解决方案

| 原因 | 症状 | 解决方案 |
|------|------|----------|
| 网络分区 | 节点间无法通信 | 修复网络，检查防火墙 |
| 节点 ID 冲突 | 日志显示 ID 冲突 | 修改重复节点的 `node_id` |
| 时钟不同步 | 选举超时时间异常 | 配置 NTP 同步 |
| 多数节点不可用 | 无法获得多数票 | 恢复离线节点 |
| 配置不一致 | 集群配置不匹配 | 统一配置文件 |

---

### 3. 写入操作返回 "not the leader"

#### 症状
```bash
$ ./client --action add --items "test"
Error: not the leader, redirect to: node1 (192.168.1.10:8080)
```

#### 诊断步骤

**步骤 1: 找到当前 Leader**
```bash
./client --server <any-node>:8080 --action stats
```

**步骤 2: 向 Leader 发送请求**
```bash
./client --server <leader-ip>:8080 --action add --items "test"
```

**步骤 3: 检查 Leader 稳定性**
```bash
# 连续查询多次，看 Leader 是否稳定
for i in {1..10}; do
    ./client --server <node>:8080 --action stats | grep "Is Leader"
    sleep 1
done
```

#### 解决方案

1. **直接连接 Leader** - 修改客户端配置指向 Leader
2. **实现自动重定向** - 客户端根据错误信息重试
3. **使用负载均衡** - 配置 L4 负载均衡，自动转发到 Leader

---

### 4. 内存使用持续增长

#### 症状
```bash
$ free -h
              total        used        free
Mem:           7.7G        6.2G        1.5G

$ ps aux | grep dbf
dbf     12345  15.2  78.5  6234567  6234567  ...
```

#### 诊断步骤

**步骤 1: 检查 Bloom Filter 大小**
```bash
./client --server localhost:8080 --action stats
# 查看 Bloom Size 和 Bloom Count
```

**步骤 2: 检查 WAL 文件**
```bash
du -sh /var/lib/dbf/data/wal/*
ls -lht /var/lib/dbf/data/wal/
```

**步骤 3: 检查快照文件**
```bash
du -sh /var/lib/dbf/data/raft/snapshots/*
ls -lh /var/lib/dbf/data/raft/snapshots/
```

**步骤 4: 分析内存分布** (需要 pprof)
```bash
# 获取内存 profile
curl http://localhost:8080/debug/pprof/heap > heap.prof

# 分析
go tool pprof heap.prof
(pprof) top10
(pprof) web
```

#### 解决方案

| 问题 | 解决方案 |
|------|----------|
| Bloom Filter 过大 | 调整 `--m` 参数，重建 Filter |
| WAL 文件堆积 | 检查快照是否正常工作 |
| 快照过多 | 清理旧快照，保留 3-5 个 |
| 内存泄漏 | 升级版本，提交 issue |

**清理命令**:
```bash
# 清理旧 WAL 文件（保留最近 3 个）
ls -t /var/lib/dbf/data/wal/*.wal | tail -n +4 | xargs rm

# 清理旧快照（保留最近 3 个）
ls -t /var/lib/dbf/data/raft/snapshots/*.snap | tail -n +4 | xargs rm
```

---

### 5. 磁盘空间不足

#### 症状
```bash
$ df -h /var/lib/dbf
Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1       100G   98G  2.0G  98% /var/lib/dbf

# 或日志中出现
# "no space left on device"
```

#### 诊断步骤

**步骤 1: 分析空间使用**
```bash
du -sh /var/lib/dbf/data/*
du -sh /var/lib/dbf/data/*/* 2>/dev/null
```

**步骤 2: 检查 WAL 文件**
```bash
ls -lh /var/lib/dbf/data/wal/
du -sh /var/lib/dbf/data/wal/
```

**步骤 3: 检查快照**
```bash
ls -lh /var/lib/dbf/data/raft/snapshots/
du -sh /var/lib/dbf/data/raft/snapshots/
```

#### 解决方案

**立即释放空间**:
```bash
# 1. 清理旧 WAL 文件
find /var/lib/dbf/data/wal -name "*.wal" -mtime +7 -delete

# 2. 清理旧快照
ls -t /var/lib/dbf/data/raft/snapshots/*.snap | tail -n +4 | xargs rm

# 3. 清理旧日志
find /var/log/dbf -name "*.log" -mtime +30 -delete
```

**长期解决方案**:
1. 配置 WAL 自动滚动
2. 限制快照数量
3. 扩容磁盘
4. 配置日志轮转

---

### 6. TLS/SSL 证书问题

#### 症状
```
failed to load certificate: open /path/to/cert.pem: no such file or directory
certificate has expired
x509: certificate signed by unknown authority
```

#### 诊断步骤

**步骤 1: 检查证书文件**
```bash
ls -la /etc/dbf/certs/
stat /etc/dbf/certs/server.crt
```

**步骤 2: 验证证书格式**
```bash
# 检查证书
openssl x509 -in /etc/dbf/certs/server.crt -text -noout

# 检查私钥
openssl rsa -in /etc/dbf/certs/server.key -check

# 检查 CA
openssl x509 -in /etc/dbf/certs/ca.crt -text -noout
```

**步骤 3: 验证证书链**
```bash
openssl verify -CAfile /etc/dbf/certs/ca.crt /etc/dbf/certs/server.crt
```

**步骤 4: 检查证书有效期**
```bash
openssl x509 -in /etc/dbf/certs/server.crt -noout -dates
```

#### 解决方案

**证书过期**:
```bash
# 生成新证书（示例，生产环境请使用正式 CA）
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout /etc/dbf/certs/server.key \
    -out /etc/dbf/certs/server.crt

# 重启服务
systemctl restart dbf-server
```

**证书路径错误**:
```bash
# 修正配置文件
vi /etc/dbf/config.yaml
# 更新证书路径

# 重启服务
systemctl restart dbf-server
```

---

### 7. 数据不一致

#### 症状
- 不同节点查询返回不同结果
- 写入后查询不到刚写入的数据

#### 诊断步骤

**步骤 1: 检查集群状态**
```bash
# 在每个节点上执行
for node in node1 node2 node3; do
    echo "=== $node ==="
    ./client --server $node:8080 --action stats
done
```

**步骤 2: 检查 Raft 日志索引**
```bash
# 查看各节点的日志索引（需要实现管理命令）
# 正常情况下应该一致或接近
```

**步骤 3: 查看同步日志**
```bash
grep -E "replicate|append|commit" /var/log/dbf/server.log | tail -50
```

#### 解决方案

**短期方案**:
```bash
# 1. 等待 Raft 自动同步（通常 <5 秒）

# 2. 如果持续不一致，重启 Follower
sudo systemctl restart dbf-server

# 3. 强制从 Leader 同步（需要实现管理命令）
```

**长期方案**:
1. 确保网络稳定
2. 监控 Raft 提交延迟
3. 配置合理的超时时间
4. 定期健康检查

---

## 🛠️ 调试工具

### 内置调试端点

```bash
# pprof 性能分析
curl http://localhost:6060/debug/pprof/heap > heap.prof
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# 指标导出
curl http://localhost:8080/metrics

# 健康检查
curl http://localhost:8080/health
```

### 日志级别调整

```bash
# 临时调整日志级别（需要实现）
curl -X POST http://localhost:8080/debug/loglevel -d "debug"

# 或修改配置文件后重启
vi /etc/dbf/config.yaml
# logging.level: debug
```

### 网络抓包

```bash
# 抓取 Raft 流量
tcpdump -i any port 8081 -w raft.pcap

# 分析
wireshark raft.pcap
# 或
tcpdump -r raft.pcap -nn
```

---

## 📞 获取帮助

### 收集诊断信息

在提交 issue 前，请收集以下信息：

```bash
# 1. 系统信息
uname -a
cat /etc/os-release

# 2. 服务状态
systemctl status dbf-server

# 3. 最近日志
journalctl -u dbf-server -n 200 --no-pager

# 4. 集群状态
./client --server localhost:8080 --action stats

# 5. 资源使用
free -h
df -h
top -bn1 | grep dbf

# 6. 网络状态
netstat -tlnp | grep dbf
```

### 联系渠道

- GitHub Issues: [提交 Bug](https://github.com/wangminggit/distributed-bloom-filter/issues)
- 文档：[完整文档](../README.md)
- 运维手册：[Operations Guide](OPERATIONS.md)

---

*文档版本: v0.1.0 | 最后更新: 2026-03-17*
