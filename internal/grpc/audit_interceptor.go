package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wangminggit/distributed-bloom-filter/internal/audit"
)

// AuditInterceptor provides audit logging for gRPC requests.
type AuditInterceptor struct {
	logger *audit.Logger
}

// NewAuditInterceptor creates a new audit interceptor.
func NewAuditInterceptor(logger *audit.Logger) *AuditInterceptor {
	return &AuditInterceptor{
		logger: logger,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that logs audit events.
func (a *AuditInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Generate request ID for tracing
		requestID := uuid.New().String()
		
		// Extract client information
		clientIP := GetClientIP(ctx)
		
		// Extract user ID from context (if available)
		userID := a.extractUserID(ctx)
		
		// Add audit info to context
		ctx = audit.ContextWithAuditInfo(ctx, requestID, clientIP, userID)
		
		// Record start time
		startTime := time.Now()
		
		// Call the actual handler
		resp, err := handler(ctx, req)
		
		// Calculate duration
		duration := time.Since(startTime)
		
		// Determine result
		result := "success"
		if err != nil {
			result = "failure"
		}
		
		// Log the audit event
		a.logRPCEvent(clientIP, userID, info.FullMethod, result, duration, err)
		
		return resp, err
	}
}

// StreamInterceptor returns a gRPC stream server interceptor that logs audit events.
func (a *AuditInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Generate request ID for tracing
		requestID := uuid.New().String()
		
		// Extract client information
		ctx := ss.Context()
		clientIP := GetClientIP(ctx)
		
		// Extract user ID from context (if available)
		userID := a.extractUserID(ctx)
		
		// Add audit info to context
		ctx = audit.ContextWithAuditInfo(ctx, requestID, clientIP, userID)
		
		// Wrap the stream to capture context
		wrappedStream := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}
		
		// Record start time
		startTime := time.Now()
		
		// Call the actual handler
		err := handler(srv, wrappedStream)
		
		// Calculate duration
		duration := time.Since(startTime)
		
		// Determine result
		result := "success"
		if err != nil {
			result = "failure"
		}
		
		// Log the audit event
		a.logRPCEvent(clientIP, userID, info.FullMethod, result, duration, err)
		
		return err
	}
}

// logRPCEvent logs an RPC event to the audit log.
func (a *AuditInterceptor) logRPCEvent(clientIP, userID, method, result string, duration time.Duration, err error) {
	if a.logger == nil {
		return
	}
	
	event := audit.NewAuditEvent(audit.EventType(methodToEventType(method)), audit.SeverityInfo).
		WithClientIP(clientIP).
		WithUserID(userID).
		WithMethod(method).
		WithResult(result).
		WithMetadata("duration_ms", duration.Milliseconds())
	
	if err != nil {
		st, _ := status.FromError(err)
		event.WithMetadata("error_code", st.Code().String()).
			WithReason(st.Message())
	}
	
	a.logger.Log(event)
}

// methodToEventType maps gRPC methods to audit event types.
func methodToEventType(method string) audit.EventType {
	// Default to a generic RPC event
	return audit.EventType("rpc." + method)
}

// extractUserID extracts the user ID from the request context or metadata.
func (a *AuditInterceptor) extractUserID(ctx context.Context) string {
	// Try to get from metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	
	// Check for user-id header
	if userIDs := md.Get("user-id"); len(userIDs) > 0 {
		return userIDs[0]
	}
	
	// Check for authorization header (could contain user info)
	if auths := md.Get("authorization"); len(auths) > 0 {
		// In production, you might decode a JWT here to extract user ID
		// For now, we just log that auth was provided
		return "authenticated"
	}
	
	return ""
}

// wrappedServerStream wraps a gRPC server stream to allow context modification.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// AuditAuthInterceptor wraps the auth interceptor to add audit logging.
type AuditAuthInterceptor struct {
	authInterceptor *AuthInterceptor
	auditLogger     *audit.Logger
}

// NewAuditAuthInterceptor creates a new audit-auth interceptor wrapper.
func NewAuditAuthInterceptor(authInterceptor *AuthInterceptor, auditLogger *audit.Logger) *AuditAuthInterceptor {
	return &AuditAuthInterceptor{
		authInterceptor: authInterceptor,
		auditLogger:     auditLogger,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that combines auth and audit.
func (a *AuditAuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		clientIP := GetClientIP(ctx)
		
		// Extract API key from request for audit logging
		apiKey := a.extractAPIKey(req)
		
		// Call the auth interceptor
		result, err := a.callAuthInterceptor(ctx, req, info, handler)
		
		// Log the authentication event
		if err != nil {
			audit.LogAuthFailure(clientIP, audit.SanitizeAPIKey(apiKey), info.FullMethod, err.Error())
		} else {
			audit.LogAuthSuccess(clientIP, audit.SanitizeAPIKey(apiKey), info.FullMethod)
		}
		
		return result, err
	}
}

// callAuthInterceptor calls the underlying auth interceptor.
func (a *AuditAuthInterceptor) callAuthInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if a.authInterceptor == nil {
		return handler(ctx, req)
	}
	
	// Get the underlying interceptor function
	interceptorFunc := a.authInterceptor.UnaryInterceptor()
	return interceptorFunc(ctx, req, info, handler)
}

// extractAPIKey extracts the API key from the request for audit logging.
func (a *AuditAuthInterceptor) extractAPIKey(req interface{}) string {
	// Try to extract API key from request if it implements GetAuth()
	if authReq, ok := req.(interface{ GetAuth() interface{} }); ok {
		if auth := authReq.GetAuth(); auth != nil {
			// Try to get ApiKey field via reflection or type assertion
			if authWithKey, ok := auth.(interface{ GetApiKey() string }); ok {
				return authWithKey.GetApiKey()
			}
		}
	}
	return ""
}

// AuditRateLimitInterceptor wraps the rate limit interceptor to add audit logging.
type AuditRateLimitInterceptor struct {
	rateInterceptor *RateLimitInterceptor
	auditLogger     *audit.Logger
}

// NewAuditRateLimitInterceptor creates a new audit-rate limit interceptor wrapper.
func NewAuditRateLimitInterceptor(rateInterceptor *RateLimitInterceptor, auditLogger *audit.Logger) *AuditRateLimitInterceptor {
	return &AuditRateLimitInterceptor{
		rateInterceptor: rateInterceptor,
		auditLogger:     auditLogger,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that combines rate limiting and audit.
func (a *AuditRateLimitInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		clientIP := GetClientIP(ctx)
		userID := a.extractUserID(ctx)
		
		// Call the rate limit interceptor
		result, err := a.callRateLimitInterceptor(ctx, req, info, handler)
		
		// Log rate limit violations
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code().String() == "ResourceExhausted" {
				audit.LogRateLimitViolation(clientIP, userID, info.FullMethod)
			}
		}
		
		return result, err
	}
}

// callRateLimitInterceptor calls the underlying rate limit interceptor.
func (a *AuditRateLimitInterceptor) callRateLimitInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if a.rateInterceptor == nil {
		return handler(ctx, req)
	}
	
	// Get the underlying interceptor function
	interceptorFunc := a.rateInterceptor.UnaryInterceptor()
	return interceptorFunc(ctx, req, info, handler)
}

// extractUserID extracts user ID from context for rate limit audit.
func (a *AuditRateLimitInterceptor) extractUserID(ctx context.Context) string {
	if _, clientIP, userID := audit.GetAuditInfoFromContext(ctx); userID != "" {
		return userID
	} else if clientIP != "" {
		return clientIP
	}
	return ""
}
