// pkg/client/tracing_client.go
package client

import (
	"context"

	constants "github.com/kubetracer/kubetracer-go/pkg/constants"

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
	ctx, span := tc.startSpanFromContext(ctx, obj, "Create "+obj.GetName())
	defer span.End()

	tc.addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Creating object", "object", obj.GetName())
	return tc.Client.Create(ctx, obj, opts...)
}

// Update adds tracing and traceID annotation around the original client's Update method
func (tc *TracingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	ctx, span := tc.startSpanFromContext(ctx, obj, "Update "+obj.GetName())
	defer span.End()

	tc.addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Updating object", "object", obj.GetName())
	return tc.Client.Update(ctx, obj, opts...)
}

// Get adds tracing around the original client's Get method
func (tc *TracingClient) GetWithSpan(ctx context.Context, key client.ObjectKey, obj client.Object) (context.Context, error) {
	// Create or retrieve the span from the context
	ctx, span := tc.startSpanFromContext(ctx, obj, "Get "+key.Name)
	defer span.End()

	tc.Logger.Info("Getting object", "object", key.Name)
	return trace.ContextWithSpan(ctx, span), tc.Client.Get(ctx, key, obj)
}

// Get adds tracing around the original client's Get method
func (tc *TracingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	// Create or retrieve the span from the context
	ctx, span := tc.startSpanFromContext(ctx, obj, "Get "+key.Name)
	defer span.End()

	tc.Logger.Info("Getting object", "object", key.Name)
	return tc.Client.Get(ctx, key, obj)
}

// Delete adds tracing around the original client's Delete method
func (tc *TracingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	ctx, span := tc.startSpanFromContext(ctx, obj, "Delete "+obj.GetName())
	defer span.End()

	tc.Logger.Info("Deleting object", "object", obj.GetName())
	return tc.Client.Delete(ctx, obj, opts...)
}

// CreateSpanID generates a new span ID and returns the updated context
func (tc *TracingClient) CreateSpanID(ctx context.Context, operationName string) context.Context {
	ctx, span := tc.Tracer.Start(ctx, operationName)
	defer span.End()
	return trace.ContextWithSpan(ctx, span)
}

// startSpanFromContext starts a new span from the context and attaches trace information to the object
func (tc *TracingClient) startSpanFromContext(ctx context.Context, obj client.Object, operationName string) (context.Context, trace.Span) {
	// Check if context already has a trace span
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: span.SpanContext().TraceID(),
		})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
		ctx, span = tc.Tracer.Start(ctx, operationName)
		return trace.ContextWithSpan(ctx, span), span
	}

	if !span.SpanContext().IsValid() {
		// No valid trace ID in context, check object annotations
		if traceID, ok := obj.GetAnnotations()[constants.TraceIDAnnotation]; ok {
			if traceIDValue, err := trace.TraceIDFromHex(traceID); err == nil {
				spanContext := trace.NewSpanContext(trace.SpanContextConfig{
					TraceID: traceIDValue,
				})
				ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
			} else {
				tc.Logger.Error(err, "Invalid trace ID", "traceID", traceID)
			}
		}
	}

	// Create a new span
	ctx, span = tc.Tracer.Start(ctx, operationName)
	return trace.ContextWithSpan(ctx, span), span
}

// addTraceIDAnnotation adds the traceID as an annotation to the object
func (tc *TracingClient) addTraceIDAnnotation(ctx context.Context, obj client.Object) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	if traceID != "" {
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(map[string]string{})
		}
		annotations := obj.GetAnnotations()
		annotations[constants.TraceIDAnnotation] = traceID
		obj.SetAnnotations(annotations)
	}
}
