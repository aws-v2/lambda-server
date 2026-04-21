package logger

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{}

// WithContext returns a child logger with trace/request fields extracted from ctx.
// Use this at the entry point of every request handler, NATS subscriber, and
// background job so all downstream log lines share the same correlation IDs.
//
//	log := logger.WithContext(ctx)
//	log.Info("processing payment", zap.String(logger.F.UserID, userID))
func WithContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(contextKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return Log
}

// InjectLogger stores a logger (already enriched with trace/request fields)
// into the context so it can be retrieved deep in the call chain.
//
//	enriched := logger.Log.With(
//	    zap.String(logger.F.TraceID,   span.TraceID),
//	    zap.String(logger.F.SpanID,    span.SpanID),
//	    zap.String(logger.F.RequestID, requestID),
//	)
//	ctx = logger.InjectLogger(ctx, enriched)
func InjectLogger(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromRequest is a convenience helper that builds an enriched logger from
// common HTTP request fields and injects it into the context in one step.
// Call this in your Gin middleware.
//
//	ctx, log := logger.FromRequest(c.Request.Context(), logger.RequestMeta{
//	    TraceID:   traceID,
//	    SpanID:    spanID,
//	    RequestID: requestID,
//	    Method:    c.Request.Method,
//	    Path:      c.FullPath(),
//	})
func FromRequest(ctx context.Context, meta RequestMeta) (context.Context, *zap.Logger) {
	l := Log.With(
		zap.String(F.TraceID,   meta.TraceID),
		zap.String(F.SpanID,    meta.SpanID),
		zap.String(F.RequestID, meta.RequestID),
		zap.String(F.HTTPMethod, meta.Method),
		zap.String(F.HTTPPath,   meta.Path),
	)
	return InjectLogger(ctx, l), l
}

// RequestMeta carries the per-request fields used to enrich a logger.
type RequestMeta struct {
	TraceID   string
	SpanID    string
	RequestID string
	Method    string
	Path      string
}