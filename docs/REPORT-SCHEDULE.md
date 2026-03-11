# David 定时汇报和代码同步机制

## 📋 汇报频率

| 类型 | 频率 | 时间 | 内容 |
|------|------|------|------|
| **日报** | 每日 | 18:00 | 当日完成的工作、测试结果、遇到的问题、明日计划 |
| **代码同步** | 实时 | 每次提交后 | 自动推送到 GitHub |
| **周报** | 每周五 | 17:00 | 本周总结、下周计划、风险评估 |

---

## 🔄 自动同步机制

### 方式 1: Git Hook（推荐）

创建 `.git/hooks/post-commit` 钩子：

```bash
#!/bin/bash
# 每次 commit 后自动 push
git push origin main
```

### 方式 2: 定时脚本

**脚本位置**: `scripts/auto-sync.sh`

**执行频率**: 每 2 小时执行一次

**功能**:
- 检查未提交的更改
- 自动 commit 并 push
- 处理远程冲突

**设置定时任务**:
```bash
# 编辑 crontab
crontab -e

# 添加定时任务（每 2 小时执行）
0 */2 * * * /home/shequ/.openclaw/workspace/projects/distributed-bloom-filter/scripts/auto-sync.sh
```

### 方式 3: GitHub Actions（推荐用于 CI/CD）

创建 `.github/workflows/sync.yml`：
```yaml
name: Auto Sync
on:
  push:
    branches: [main]
  schedule:
    - cron: '0 */2 * * *'  # 每 2 小时

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run tests
        run: go test ./...
      - name: Build
        run: go build ./...
```

---

## 📝 David 日报模板

```markdown
## David 开发日报

**日期**: 2026-03-11
**项目**: Distributed Bloom Filter

### ✅ 今日完成

1. **模块名称** - 进度 XX%
   - 具体完成的功能点
   - 测试结果（如：5/5 测试通过）

2. **模块名称** - 进度 XX%
   - ...

### 🔄 进行中

1. **模块名称** - 进度 XX%
   - 当前状态
   - 预计完成时间

### 📊 代码统计

- 新增文件：X 个
- 新增代码：X 行
- 提交次数：X 次
- GitHub 同步：✅ 已推送

### ⚠️ 遇到的问题

1. **问题描述**
   - 影响：...
   - 解决方案：...
   - 是否需要协助：是/否

### 📅 明日计划

1. 任务 1（优先级：高）
2. 任务 2（优先级：中）
3. 任务 3（优先级：低）

### 💡 备注

其他需要说明的事项...
```

---

## 📅 定时汇报时间表

| 时间 | 事件 | 参与者 |
|------|------|--------|
| **每日 09:00** | 站会（异步） | 全员 |
| **每日 18:00** | David 日报 | David → Guawa → wm |
| **每 2 小时** | 代码自动同步 | 自动脚本 |
| **每周五 17:00** | 周报复盘 | 全员 + wm |

---

## 🚀 GitHub 同步策略

### 同步触发条件

1. **代码提交后** - 立即 push
2. **定时检查** - 每 2 小时检查未提交更改
3. **日报前** - 确保当天代码已推送

### 分支策略

```
main (保护分支)
  │
  ├── feature/raft-integration
  ├── feature/wal-encryption
  └── feature/metadata-service
```

**规则**:
- `main` 分支：只能通过 PR 合并
- 特性分支：直接 push
- 每天至少一次同步到 `main`

---

## 📧 汇报渠道

| 类型 | 渠道 | 接收人 |
|------|------|--------|
| 日报 | Feishu / GitHub Issue | wm, Alex, Sarah |
| 代码同步 | GitHub Push | 自动通知 |
| 紧急问题 | Feishu 即时消息 | wm, Alex |
| 周报 | Feishu / Email | wm, Alex, Sarah |

---

## 🔔 提醒设置

### 使用 Feishu 机器人

创建 Feishu 群机器人，定时发送提醒：

```yaml
# 日报提醒 - 每天 17:30
- 时间：17:30
  内容："@David 该提交今日日报了！"

# 代码同步检查 - 每 2 小时
- 时间：0 */2 * * *
  内容："@David 检查代码是否已同步到 GitHub"
```

---

## 📊 进度追踪看板

使用 GitHub Projects 或 Feishu 多维表格：

| 任务 | 负责人 | 状态 | 优先级 | 截止日期 |
|------|--------|------|--------|----------|
| Raft 集成 | David | In Progress | P0 | 3/15 |
| WAL 加密 | David | Todo | P0 | 3/16 |
| 元数据服务 | David | Todo | P1 | 3/17 |

---

## ✅ 检查清单

### 每日检查

- [ ] 代码已提交并推送到 GitHub
- [ ] 单元测试通过
- [ ] 日报已发送
- [ ] 无阻塞性问题

### 每周检查

- [ ] 周报已发送
- [ ] 代码覆盖率检查
- [ ] 性能基准测试
- [ ] 下周计划已制定

---

*Last updated: 2026-03-11*
