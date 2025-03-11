package client

import (
	"context"

	constants "github.com/kubetracer/kubetracer-go/pkg/constants"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TracingClient wraps the Kubernetes client to add tracing functionality
type tracingClient struct {
	client.Client
	trace.Tracer
	Logger logr.Logger
}

type tracingStatusClient struct {
	client.StatusWriter
	trace.Tracer
	Logger logr.Logger
}

type TracingClient interface {
	client.Client
	trace.Tracer
	// We use this to which calls client.Client Get
	GetWithSpan(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (context.Context, error)
}

var _ TracingClient = (*tracingClient)(nil)
var _ client.StatusWriter = (*tracingStatusClient)(nil)

// NewTracingClient initializes and returns a new TracingClient
func NewTracingClient(c client.Client, t trace.Tracer, l logr.Logger) TracingClient {
	return &tracingClient{
		Client: c,
		Tracer: t,
		Logger: l,
	}
}

// Create adds tracing and traceID annotation around the original client's Create method
func (tc *tracingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Create "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Creating object", "object", obj.GetName())
	return tc.Client.Create(ctx, obj, opts...)
}

// Update adds tracing and traceID annotation around the original client's Update method
func (tc *tracingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Update "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Updating object", "object", obj.GetName())
	return tc.Client.Update(ctx, obj, opts...)
}

// Get adds tracing around the original client's Get method
func (tc *tracingClient) GetWithSpan(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (context.Context, error) {
	// Create or retrieve the span from the context
	err := tc.Client.Get(ctx, key, obj, opts...)
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Get "+key.Name)
	defer span.End()

	tc.Logger.Info("Getting object", "object", key.Name)
	return trace.ContextWithSpan(ctx, span), err
}

// Get adds tracing around the original client's Get method
func (tc *tracingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// Create or retrieve the span from the context
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Get "+key.Name)
	defer span.End()

	tc.Logger.Info("Getting object", "object", key.Name)
	return tc.Client.Get(ctx, key, obj)
}

func (tc *tracingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	ctx, span := startSpanFromContextList(ctx, tc.Logger, tc.Tracer, list, "List ")
	defer span.End()

	tc.Logger.Info("Getting List")
	return tc.Client.List(ctx, list, opts...)
}

// Patch  adds tracing and traceID annotation around the original client's Patch method
func (tc *tracingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Patch "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Patching object", "object", obj.GetName())
	return tc.Client.Patch(ctx, obj, patch, opts...)
}

// Delete adds tracing around the original client's Delete method
func (tc *tracingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "Delete "+obj.GetName())
	defer span.End()

	tc.Logger.Info("Deleting object", "object", obj.GetName())
	return tc.Client.Delete(ctx, obj, opts...)
}

func (tc *tracingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, "DeleteAllOf "+obj.GetName())
	defer span.End()

	tc.Logger.Info("Deleting all of object", "object", obj.GetName())
	return tc.Client.DeleteAllOf(ctx, obj, opts...)

}

func (tc *tracingClient) Status() client.StatusWriter {
	return &tracingStatusClient{
		Logger:       tc.Logger,
		StatusWriter: tc.Client.Status(),
		Tracer:       tc.Tracer,
	}
}

func (ts *tracingStatusClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, "StatusUpdate "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	ts.Logger.Info("updating object", "object", obj.GetName())
	return ts.StatusWriter.Update(ctx, obj, opts...)
}

func (ts *tracingStatusClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, "StatusPatch "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	return ts.StatusWriter.Patch(ctx, obj, patch, opts...)
}

func (ts *tracingStatusClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, "StatusCreate "+obj.GetName())
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	return ts.StatusWriter.Create(ctx, obj, subResource, opts...)
}

// CreateSpanID generates a new span ID and returns the updated context
func (tc *tracingClient) CreateSpanID(ctx context.Context, operationName string) context.Context {
	ctx, span := tc.Tracer.Start(ctx, operationName)
	defer span.End()
	return trace.ContextWithSpan(ctx, span)
}

// startSpanFromContext starts a new span from the context and attaches trace information to the object
func startSpanFromContext(ctx context.Context, logger logr.Logger, tracer trace.Tracer, obj client.Object, operationName string) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: span.SpanContext().TraceID(),
		})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
		ctx, span = tracer.Start(ctx, operationName)
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
				logger.Error(err, "Invalid trace ID", "traceID", traceID)
			}
		}
	}

	// Create a new span
	ctx, span = tracer.Start(ctx, operationName)
	return trace.ContextWithSpan(ctx, span), span
}

func startSpanFromContextList(ctx context.Context, logger logr.Logger, tracer trace.Tracer, obj client.ObjectList, operationName string) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: span.SpanContext().TraceID(),
		})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
		ctx, span = tracer.Start(ctx, operationName)
		return trace.ContextWithSpan(ctx, span), span
	}

	// Create a new span
	ctx, span = tracer.Start(ctx, operationName)
	return trace.ContextWithSpan(ctx, span), span
}

// addTraceIDAnnotation adds the traceID as an annotation to the object
func addTraceIDAnnotation(ctx context.Context, obj client.Object) {
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
