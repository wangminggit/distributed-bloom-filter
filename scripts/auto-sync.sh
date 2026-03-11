#!/bin/bash

# DBF 项目自动同步脚本
# 用途：检查代码更新并推送到 GitHub

set -e

PROJECT_DIR="/home/shequ/.openclaw/workspace/projects/distributed-bloom-filter"
LOG_FILE="/tmp/dbf-sync-$(date +%Y%m%d).log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

cd "$PROJECT_DIR"

log "=== DBF 项目 Git 同步开始 ==="

# 检查是否有未提交的更改
if git status --porcelain | grep -q .; then
    log "检测到未提交的更改"
    
    # 添加所有更改
    git add -A
    
    # 获取更改统计
    CHANGED=$(git diff --cached --stat)
    log "变更文件:\n$CHANGED"
    
    # 提交
    git commit -m "Auto-sync: $(date '+%Y-%m-%d %H:%M')"
    
    # 推送
    if git push origin main; then
        log "✅ 推送成功"
    else
        log "❌ 推送失败，可能远程有更新，先拉取"
        git pull --rebase origin main
        git push origin main
        log "✅ 重新推送成功"
    fi
else
    log "✅ 没有未提交的更改"
fi

# 检查远程更新
REMOTE_CHANGES=$(git rev-list HEAD..origin/main --count 2>/dev/null || echo "0")
if [ "$REMOTE_CHANGES" -gt 0 ]; then
    log "⚠️ 远程有 $REMOTE_CHANGES 个新提交，正在拉取"
    git pull origin main
fi

log "=== DBF 项目 Git 同步完成 ==="
log ""
