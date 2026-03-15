package audit_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/audit"
)

// 示例：审计日志系统使用演示
func main() {
	fmt.Println("=== Audit Logging System Demo ===")
	fmt.Println()

	// 1. 初始化审计日志器
	fmt.Println("1. Initializing audit logger...")
	config := audit.LoggerConfig{
		LogDir:        "logs/audit",
		MaxFileSize:   10 * 1024 * 1024, // 10MB
		MaxAge:        30 * 24 * time.Hour,
		BufferSize:    1000,
		FlushInterval: 5 * time.Second,
		EnableConsole: true, // 演示时启用控制台输出
	}

	logger, err := audit.NewLogger(config)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	fmt.Println("   ✓ Audit logger initialized")
	fmt.Println()

	// 2. 使用便捷函数记录事件
	fmt.Println("2. Logging events with helper functions...")

	// 认证成功
	audit.LogAuthSuccess("192.168.1.100", "user123", "/proto.DBFService/Check")

	// 认证失败
	audit.LogAuthFailure("192.168.1.101", "attacker", "/proto.DBFService/Check", "Invalid API key")

	// 限流违规
	audit.LogRateLimitViolation("192.168.1.102", "user456", "/proto.DBFService/Add")

	// 权限变更
	audit.LogPermissionChange("192.168.1.1", "admin", "grant", "user789", "write")

	// 配置修改
	audit.LogConfigChange("192.168.1.1", "admin", "rate_limit", 100, 200)

	fmt.Println("   ✓ Events logged")
	fmt.Println()

	// 3. 使用构建器模式创建自定义事件
	fmt.Println("3. Creating custom events with builder pattern...")

	event := audit.NewAuditEvent(audit.EventConfigModified, audit.SeverityWarning).
		WithClientIP("192.168.1.1").
		WithUserID("admin").
		WithMethod("/admin/config/update").
		WithResult("success").
		WithReason("Updated system configuration").
		WithRequestID("req-demo-001").
		WithMetadata("config_key", "max_connections").
		WithMetadata("old_value", 1000).
		WithMetadata("new_value", 2000)

	logger.Log(event)
	fmt.Println("   ✓ Custom event created")
	fmt.Println()

	// 4. 上下文传递示例
	fmt.Println("4. Demonstrating context propagation...")

	ctx := audit.ContextWithAuditInfo(
		context.Background(),
		"req-demo-002",
		"192.168.1.100",
		"user123",
	)

	requestID, clientIP, userID := audit.GetAuditInfoFromContext(ctx)
	fmt.Printf("   RequestID: %s\n", requestID)
	fmt.Printf("   ClientIP: %s\n", clientIP)
	fmt.Printf("   UserID: %s\n\n", userID)

	// 5. 敏感信息脱敏示例
	fmt.Println("5. Demonstrating sensitive data sanitization...")

	apiKey := "sk-1234567890abcdef"
	password := "mysecretpassword123"
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

	fmt.Printf("   Original API Key: %s\n", apiKey)
	fmt.Printf("   Sanitized API Key: %s\n\n", audit.SanitizeAPIKey(apiKey))

	fmt.Printf("   Original Password: %s\n", password)
	fmt.Printf("   Sanitized Password: %s\n\n", audit.SanitizeValue(password))

	fmt.Printf("   Original Token: %s\n", token)
	fmt.Printf("   Sanitized Token: %s\n\n", audit.SanitizeValue(token))

	// 6. 同步写入示例（关键事件）
	fmt.Println("6. Demonstrating synchronous write for critical events...")

	criticalEvent := audit.NewAuditEvent(audit.EventAuthFailure, audit.SeverityCritical).
		WithClientIP("10.0.0.1").
		WithUserID("unknown").
		WithMethod("/admin/login").
		WithResult("failure").
		WithReason("Multiple failed login attempts - possible brute force attack").
		WithMetadata("attempt_count", 10)

	if err := logger.LogSync(criticalEvent); err != nil {
		log.Printf("Failed to write critical event: %v", err)
	}
	fmt.Println("   ✓ Critical event written synchronously")
	fmt.Println()

	// 7. 获取日志文件列表
	fmt.Println("7. Listing audit log files...")
	files, err := audit.GetLogFiles(config.LogDir)
	if err != nil {
		log.Printf("Failed to get log files: %v", err)
	} else {
		for i, file := range files {
			fmt.Printf("   [%d] %s\n", i+1, file)
		}
	}

	// 等待异步写入完成
	fmt.Println("\n8. Waiting for async writes to complete...")
	time.Sleep(1 * time.Second)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Check the logs/audit/ directory for generated log files.")
}
