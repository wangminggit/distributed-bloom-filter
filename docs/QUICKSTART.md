# DBF 快速入门 (Quick Start)

**5 分钟上手 Distributed Bloom Filter**

---

## 📦 安装

### 方式 1: 下载二进制 (推荐)

```bash
# 下载最新版本
wget https://github.com/wangminggit/distributed-bloom-filter/releases/download/v0.1.0/dbf-server-linux-amd64
wget https://github.com/wangminggit/distributed-bloom-filter/releases/download/v0.1.0/dbf-client-linux-amd64

# 添加执行权限
chmod +x dbf-server-linux-amd64 dbf-client-linux-amd64

# 移动到 PATH
sudo mv dbf-server-linux-amd64 /usr/local/bin/dbf-server
sudo mv dbf-client-linux-amd64 /usr/local/bin/dbf-client
```

### 方式 2: 从源码编译

```bash
# 克隆仓库
git clone https://github.com/wangminggit/distributed-bloom-filter.git
cd distributed-bloom-filter

# 编译
make build

# 验证安装
./server --version
./client --version
```

### 方式 3: Docker

```bash
# 拉取镜像
docker pull wangming/dbf:v0.1.0

# 运行
docker run -d --name dbf \
  -p 8080:8080 -p 8081:8081 \
  -v /data/dbf:/var/lib/dbf/data \
  wangming/dbf:v0.1.0
```

---

## 🚀 快速开始

### 单节点模式

**1. 启动服务器**
```bash
# 作为集群第一个节点启动（bootstrap）
dbf-server --bootstrap --node-id node1
```

**2. 添加元素**
```bash
# 添加单个元素
dbf-client --action add --items "hello"

# 添加多个元素
dbf-client --action add --items "world,foo,bar"
```

**3. 查询元素**
```bash
# 检查单个元素
dbf-client --action contains --items "hello"
# 输出：✓ Contains: hello

# 批量检查
dbf-client --action batch-contains --items "hello,world,test"
```

**4. 删除元素**
```bash
dbf-client --action remove --items "hello"
```

**5. 查看统计**
```bash
dbf-client --action stats
```

输出示例:
```
Server Statistics:
  Node ID:      node1
  Is Leader:    true
  Raft State:   Leader
  Leader:       node1
  Bloom Size:   10000 bits
  Bloom K:      3
  Bloom Count:  ~5 items
  Raft Port:    8081
```

---

## 🌐 集群模式

### 3 节点集群示例

**节点 1 (Leader)**:
```bash
# 在 server1 上
dbf-server --bootstrap --node-id node1 --port 8080 --raft-port 8081
```

**节点 2 (Follower)**:
```bash
# 在 server2 上
dbf-server --node-id node2 --port 8080 --raft-port 8081
```

**节点 3 (Follower)**:
```bash
# 在 server3 上
dbf-server --node-id node3 --port 8080 --raft-port 8081
```

### 验证集群状态

```bash
# 在任意节点执行
dbf-client --server server1:8080 --action stats
dbf-client --server server2:8080 --action stats
dbf-client --server server3:8080 --action stats
```

### 写入数据

```bash
# 写入到 Leader（自动重定向）
dbf-client --server server1:8080 --action add --items "cluster-data"

# 从任意节点读取（本地读）
dbf-client --server server2:8080 --action contains --items "cluster-data"
dbf-client --server server3:8080 --action contains --items "cluster-data"
```

---

## ⚙️ 配置选项

### 常用命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--port` | 8080 | gRPC 服务端口 |
| `--raft-port` | 8081 | Raft 通信端口 |
| `--node-id` | node1 | 节点唯一标识 |
| `--data-dir` | ./data | 数据目录 |
| `--bootstrap` | false | 是否作为集群第一个节点 |
| `--m` | 10000 | Bloom Filter 大小 (bits) |
| `--k` | 3 | Hash 函数数量 |

### 示例配置

```bash
# 生产环境配置示例
dbf-server \
  --bootstrap \
  --node-id prod-node1 \
  --port 8080 \
  --raft-port 8081 \
  --data-dir /var/lib/dbf/data \
  --m 1000000 \
  --k 5
```

---

## 🔒 安全配置

### 启用 mTLS

**1. 生成证书**
```bash
# 生成 CA
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -out ca.crt

# 生成服务器证书
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -out server.crt -days 365
```

**2. 启动服务器**
```bash
dbf-server \
  --bootstrap \
  --enable-mtls \
  --ca-cert ca.crt \
  --server-cert server.crt \
  --server-key server.key
```

### 启用 Token 认证

```bash
dbf-server \
  --bootstrap \
  --enable-token-auth \
  --jwt-secret "your-secret-key-here" \
  --token-expiry 24h
```

---

## 📊 监控

### Prometheus 指标

```bash
# 查看指标
curl http://localhost:8080/metrics
```

关键指标:
- `dbf_raft_is_leader` - 是否为 Leader
- `dbf_bloom_element_count` - 元素数量
- `dbf_operation_duration_seconds` - 操作延迟
- `dbf_wal_size_bytes` - WAL 大小

### 健康检查

```bash
# HTTP 健康检查
curl http://localhost:8080/health

# 使用 CLI
dbf-client --action stats
```

---

## 🧪 测试

### 功能测试

```bash
# 添加测试数据
dbf-client --action batch-add --items "a,b,c,d,e"

# 验证存在
dbf-client --action batch-contains --items "a,b,c,d,e,f"
# 预期：a,b,c,d,e 存在，f 不存在

# 删除
dbf-client --action remove --items "a,b"

# 再次验证
dbf-client --action contains --items "a"
# 预期：不存在
```

### 性能测试

```bash
# 使用内置基准测试
cd distributed-bloom-filter
go test -bench=. ./tests/performance/

# 示例输出:
# BenchmarkAddPerformance-8    1000000    38.5 ns/op
# BenchmarkContainsPerformance-8 2000000   32.1 ns/op
```

---

## ❓ 常见问题

### Q: 误判率是多少？

A: 默认配置下 (m=10000, k=3)，添加 1000 个元素时误判率约 3%。调整参数可降低误判率：
- 增加 `--m` (Bloom Filter 大小)
- 优化 `--k` (Hash 函数数量)

### Q: 数据会丢失吗？

A: DBF 通过以下机制保证数据持久性:
- WAL (Write-Ahead Log) 记录所有写操作
- Raft 共识确保多副本
- 定期快照

### Q: 如何备份数据？

A: 备份数据目录即可:
```bash
tar -czf dbf-backup.tar.gz /var/lib/dbf/data
```

### Q: 支持多少并发？

A: 单节点可支持 10 万+ QPS，集群模式可线性扩展。

---

## 📚 下一步

- [API 参考文档](API.md) - 完整 gRPC API 说明
- [运维手册](OPERATIONS.md) - 生产环境运维指南
- [故障排查](TROUBLESHOOTING.md) - 常见问题解决
- [架构文档](../ARCHITECTURE.md) - 系统架构详解

---

## 🆘 获取帮助

- 📖 [完整文档](../README.md)
- 🐛 [提交 Issue](https://github.com/wangminggit/distributed-bloom-filter/issues)
- 💬 [讨论区](https://github.com/wangminggit/distributed-bloom-filter/discussions)

---

*文档版本：v0.1.0 | 最后更新：2026-03-17*
