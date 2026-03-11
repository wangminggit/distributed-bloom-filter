# Contributing to Distributed Bloom Filter

首先，感谢你考虑为 DBF 项目做出贡献！

## 行为准则

本项目遵循 [Contributor Covenant](https://www.contributor-covenant.org/) 行为准则。
请确保你的言行符合开放、友好、包容的社区氛围。

## 如何贡献

### 报告 Bug

1. 首先检查 [Issues](https://github.com/yourorg/dbf/issues) 是否已有人报告
2. 如果没有，创建一个新的 Issue，包含：
   - 清晰的标题和描述
   - 复现步骤
   - 预期行为 vs 实际行为
   - 环境信息（Go 版本、OS、K8s 版本等）
   - 日志输出（如果适用）

### 提出新功能

1. 先在 Issue 中讨论你的想法
2. 说明使用场景和必要性
3. 如果获得认可，可以开始实现

### 提交代码

#### 1. Fork 项目

```bash
git clone https://github.com/yourorg/dbf.git
cd dbf
```

#### 2. 创建分支

```bash
git checkout -b feature/your-feature-name
# 或
git checkout -b fix/issue-123
```

**分支命名规范**:
- `feature/xxx` - 新功能
- `fix/xxx` - Bug 修复
- `docs/xxx` - 文档更新
- `test/xxx` - 测试相关
- `refactor/xxx` - 重构

#### 3. 开发

```bash
# 安装依赖
go mod download

# 运行测试
go test ./...

# 代码格式化
go fmt ./...
go vet ./...

# 构建
go build -o dbf ./cmd/server
```

**代码规范**:
- 遵循 [Effective Go](https://golang.org/doc/effective_go.html)
- 函数要有单元测试
- 添加必要的注释
- 错误处理要完整

#### 4. 提交

```bash
git add .
git commit -m "feat: add batch delete support"
```

**Commit 信息规范** (Conventional Commits):
```
feat: 新功能
fix: Bug 修复
docs: 文档更新
style: 代码格式（不影响功能）
refactor: 重构
test: 测试相关
chore: 构建/工具相关
```

#### 5. 推送并创建 PR

```bash
git push origin feature/your-feature-name
```

然后在 GitHub 上创建 Pull Request。

**PR 描述模板**:
```markdown
## 变更说明
简要描述你的变更内容

## 相关 Issue
Fixes #123

## 测试
- [ ] 已添加单元测试
- [ ] 已通过所有现有测试
- [ ] 已手动测试

## 检查清单
- [ ] 代码已格式化 (go fmt)
- [ ] 已通过 go vet
- [ ] 文档已更新（如需要）
- [ ] 变更日志已更新（如需要）
```

## 开发环境设置

### 前置要求

- Go 1.21+
- Docker 20+
- Kubernetes 1.25+ (可选，用于集成测试)
- Make (可选)

### 本地运行

```bash
# 单机模式
go run ./cmd/server --mode=standalone

# 或使用 Docker
docker-compose up
```

### 运行测试

```bash
# 单元测试
go test ./...

# 集成测试
go test -tags=integration ./...

# 性能测试
go test -bench=. ./...

# 测试覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## 代码审查

所有 PR 都需要经过至少一位维护者的审查。审查要点：

- 代码质量
- 测试覆盖
- 文档完整性
- 性能影响
- 向后兼容性

## 发布流程

1. 更新 [CHANGELOG.md](CHANGELOG.md)
2. 更新版本号
3. 创建 Git tag
4. 构建并发布 Docker 镜像
5. 发布 GitHub Release

## 问题？

有任何问题，欢迎在 Issue 中提问或加入我们的讨论群。

---

再次感谢你的贡献！🎉
