#!/bin/bash

# GitHub 推送通知脚本
# 用途：推送成功后发送通知到 Feishu

WEBHOOK_URL="${FEISHU_WEBHOOK_URL:-}"  # 从环境变量读取

if [ -z "$WEBHOOK_URL" ]; then
    echo "⚠️ FEISHU_WEBHOOK_URL 未设置，跳过通知"
    exit 0
fi

PROJECT_DIR="/home/shequ/.openclaw/workspace/projects/distributed-bloom-filter"
cd "$PROJECT_DIR"

# 获取最新提交信息
COMMIT_HASH=$(git rev-parse --short HEAD)
COMMIT_MSG=$(git log -1 --pretty=%s)
COMMIT_AUTHOR=$(git log -1 --pretty=%an)
COMMIT_TIME=$(git log -1 --pretty=%ai)
FILES_CHANGED=$(git diff-tree --no-commit-id --name-only -r HEAD | wc -l)
LINES_ADDED=$(git diff-tree --no-commit-id --stat -r HEAD | tail -1 | awk '{print $4}' | tr -d '+')
LINES_DELETED=$(git diff-tree --no-commit-id --stat -r HEAD | tail -1 | awk '{print $6}' | tr -d '-')

# 构建通知消息
CONTENT=$(cat <<EOF
{
    "msg_type": "interactive",
    "card": {
        "header": {
            "title": {
                "tag": "plain_text",
                "content": "🚀 DBF 代码已同步到 GitHub"
            },
            "template": "blue"
        },
        "elements": [
            {
                "tag": "div",
                "text": {
                    "tag": "lark_md",
                    "content": "**提交信息**\n- 哈希：\`$COMMIT_HASH\`\n- 作者：$COMMIT_AUTHOR\n- 时间：$COMMIT_TIME\n- 消息：$COMMIT_MSG"
                }
            },
            {
                "tag": "hr"
            },
            {
                "tag": "div",
                "text": {
                    "tag": "lark_md",
                    "content": "**变更统计**\n- 修改文件：$FILES_CHANGED 个\n- 新增行数：+$LINES_ADDED\n- 删除行数：-$LINES_DELETED"
                }
            },
            {
                "tag": "action",
                "actions": [
                    {
                        "tag": "button",
                        "text": {
                            "tag": "plain_text",
                            "content": "📝 查看提交"
                        },
                        "url": "https://github.com/wangminggit/distributed-bloom-filter/commit/$COMMIT_HASH",
                        "type": "primary"
                    },
                    {
                        "tag": "button",
                        "text": {
                            "tag": "plain_text",
                            "content": "📂 查看仓库"
                        },
                        "url": "https://github.com/wangminggit/distributed-bloom-filter",
                        "type": "default"
                    }
                ]
            }
        ]
    }
}
EOF
)

# 发送通知
curl -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -d "$CONTENT" \
  -s \
  -o /dev/null

if [ $? -eq 0 ]; then
    echo "✅ 通知已发送"
else
    echo "❌ 通知发送失败"
fi
