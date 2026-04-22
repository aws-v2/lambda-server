package logger

// fields holds the canonical JSON key names for every structured log field.
// All microservices must use these constants — never raw string literals —
// so Kibana index mappings and Logstash grok patterns stay in sync.
//
// Usage:
//
//	logger.Log.Info("request completed",
//	    zap.String(logger.F.TraceID,    traceID),
//	    zap.String(logger.F.RequestID,  reqID),
//	    zap.Int(logger.F.HTTPStatus,    200),
//	    zap.Int64(logger.F.DurationMS,  42),
//	)
var F = fields{
	// ── Identity (baked in at Init, never set manually) ──────────────────
	Timestamp: "@timestamp", // ISO-8601, parsed by Logstash date filter
	Level:     "level",
	Message:   "message",
	Caller:    "caller",
	Service:   "service",
	Profile:   "profile",
	Region:    "region",

	// ── Distributed tracing (OpenTelemetry standard names) ───────────────
	TraceID: "trace.id",
	SpanID:  "span.id",

	// ── Request correlation ───────────────────────────────────────────────
	RequestID: "request.id",

	// ── HTTP fields (use for middleware logs) ─────────────────────────────
	HTTPMethod:     "http.method",
	HTTPPath:       "http.path",
	HTTPStatus:     "http.status",
	HTTPUserAgent:  "http.user_agent",
	DurationMS:     "duration_ms",

	// ── Domain / business context (optional, set per use-case) ───────────
	UserID:    "user.id",
	Domain:    "domain",
	EventType: "event.type",  // e.g. "lambda.executed", "scale.triggered"

	// ── Error details ─────────────────────────────────────────────────────
	ErrorMsg:   "error.message",
	ErrorKind:  "error.kind",   // e.g. "db_error", "nats_timeout"
	ErrorStack: "error.stack",  // only logged at error level

Action: "action",	
Env: "env",
	



}

type fields struct {
	Timestamp string
	Level     string
	Message   string
	Caller    string
	Service   string
	Profile   string
	Region    string

	TraceID string
	SpanID  string

	RequestID string

	HTTPMethod    string
	HTTPPath      string
	HTTPStatus    string
	HTTPUserAgent string
	DurationMS    string

	UserID    string
	Domain    string
	EventType string

	ErrorMsg   string
	ErrorKind  string
	ErrorStack string


	Action string
	Env string
	
}