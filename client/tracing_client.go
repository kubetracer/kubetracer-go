package client

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TracingClient wraps the Kubernetes client to add tracing functionality
type TracingClient struct {
	client.Client
	Tracer trace.Tracer
	Logger logr.Logger
}

// NewTracingClient initializes and returns a new TracingClient
func NewTracingClient(c client.Client, t trace.Tracer, l logr.Logger) *TracingClient {
	return &TracingClient{
		Client: c,
		Tracer: t,
		Logger: l,
	}
}

// Create adds tracing and traceID annotation around the original client's Create method
func (tc *TracingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	ctx, span := tc.startSpanFromContext(ctx, obj, "Create")
	defer span.End()

	tc.addTraceIDAnnotation(ctx, obj)

	tc.Logger.Info("Creating object", "object", obj.GetName())
	return tc.Client.Create(ctx, obj, opts...)
}

// Update adds tracing and traceID annotation around the original client's Update method
func (tc *TracingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	ctx, span := tc.startSpanFromContext(ctx, obj, "Update")
	defer span.End()

	tc.addTraceIDAnnotation(ctx, obj)

	tc.Logger.Info("Updating object", "object", obj.GetName())
	return tc.Client.Update(ctx, obj, opts...)
}

// startSpanFromContext starts a new span from the context and attaches trace information to the object
func (tc *TracingClient) startSpanFromContext(ctx context.Context, obj client.Object, operationName string) (context.Context, trace.Span) {
	var span trace.Span
	traceID, ok := obj.GetAnnotations()["kubetracer.io/trace-id"]
	if ok {
		traceIDValue, err := trace.TraceIDFromHex(traceID)
		if err != nil {
			tc.Logger.Error(err, "Invalid trace ID", "traceID", traceID)
			return ctx, span
		}
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceIDValue})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
	}
	ctx, span = tc.Tracer.Start(ctx, operationName)
	return ctx, span
}

// addTraceIDAnnotation adds the traceID as an annotation to the object
func (tc *TracingClient) addTraceIDAnnotation(ctx context.Context, obj client.Object) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	if traceID != "" {
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(map[string]string{})
		}
		obj.GetAnnotations()["kubetracer.io/trace-id"] = traceID
	}
}
