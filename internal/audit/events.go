package audit

import "time"

// EventType represents the type of security event being audited.
type EventType string

const (
	// Authentication events
	EventAuthSuccess EventType = "auth.success"
	EventAuthFailure EventType = "auth.failure"
	
	// Rate limiting events
	EventRateLimitViolated EventType = "ratelimit.violated"
	
	// Permission events
	EventPermissionChanged EventType = "permission.changed"
	EventPermissionGranted EventType = "permission.granted"
	EventPermissionRevoked EventType = "permission.revoked"
	
	// Configuration events
	EventConfigModified EventType = "config.modified"
	EventConfigCreated  EventType = "config.created"
	EventConfigDeleted  EventType = "config.deleted"
	
	// System events
	EventSystemStart   EventType = "system.start"
	EventSystemStop    EventType = "system.stop"
	EventSystemRestart EventType = "system.restart"
)

// EventSeverity represents the severity level of an audit event.
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "INFO"
	SeverityWarning  EventSeverity = "WARNING"
	SeverityError    EventSeverity = "ERROR"
	SeverityCritical EventSeverity = "CRITICAL"
)

// AuditEvent represents a single audit log entry.
type AuditEvent struct {
	// Timestamp is when the event occurred (Unix timestamp in milliseconds)
	Timestamp int64 `json:"timestamp"`
	
	// Time is the human-readable timestamp (ISO 8601 format)
	Time string `json:"time"`
	
	// EventType is the type of event
	EventType EventType `json:"event_type"`
	
	// Severity is the severity level of the event
	Severity EventSeverity `json:"severity"`
	
	// ClientIP is the IP address of the client that triggered the event
	ClientIP string `json:"client_ip,omitempty"`
	
	// UserID is the identifier of the user associated with the event
	UserID string `json:"user_id,omitempty"`
	
	// Method is the gRPC method or API endpoint that was called
	Method string `json:"method,omitempty"`
	
	// Result indicates whether the operation succeeded or failed
	Result string `json:"result"`
	
	// Reason provides additional context about why the event occurred
	Reason string `json:"reason,omitempty"`
	
	// Metadata contains additional event-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// RequestID is a unique identifier for tracing the request
	RequestID string `json:"request_id,omitempty"`
}

// NewAuditEvent creates a new audit event with the current timestamp.
func NewAuditEvent(eventType EventType, severity EventSeverity) *AuditEvent {
	now := time.Now()
	return &AuditEvent{
		Timestamp: now.UnixMilli(),
		Time:      now.Format(time.RFC3339),
		EventType: eventType,
		Severity:  severity,
		Metadata:  make(map[string]interface{}),
	}
}

// WithClientIP sets the client IP address.
func (e *AuditEvent) WithClientIP(ip string) *AuditEvent {
	e.ClientIP = ip
	return e
}

// WithUserID sets the user ID.
func (e *AuditEvent) WithUserID(userID string) *AuditEvent {
	e.UserID = userID
	return e
}

// WithMethod sets the method/endpoint.
func (e *AuditEvent) WithMethod(method string) *AuditEvent {
	e.Method = method
	return e
}

// WithResult sets the result (success/failure).
func (e *AuditEvent) WithResult(result string) *AuditEvent {
	e.Result = result
	return e
}

// WithReason sets the reason/description.
func (e *AuditEvent) WithReason(reason string) *AuditEvent {
	e.Reason = reason
	return e
}

// WithMetadata adds metadata to the event.
func (e *AuditEvent) WithMetadata(key string, value interface{}) *AuditEvent {
	e.Metadata[key] = value
	return e
}

// WithRequestID sets the request ID for tracing.
func (e *AuditEvent) WithRequestID(requestID string) *AuditEvent {
	e.RequestID = requestID
	return e
}

// GetDefaultSeverity returns the default severity for an event type.
func GetDefaultSeverity(eventType EventType) EventSeverity {
	switch eventType {
	case EventAuthSuccess:
		return SeverityInfo
	case EventAuthFailure:
		return SeverityWarning
	case EventRateLimitViolated:
		return SeverityWarning
	case EventPermissionChanged, EventPermissionGranted, EventPermissionRevoked:
		return SeverityWarning
	case EventConfigModified, EventConfigCreated, EventConfigDeleted:
		return SeverityWarning
	case EventSystemStart, EventSystemRestart:
		return SeverityInfo
	case EventSystemStop:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}
