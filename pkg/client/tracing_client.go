package client

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	constants "github.com/kubetracer/kubetracer-go/pkg/constants"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// TracingClient wraps the Kubernetes client to add tracing functionality
type tracingClient struct {
	scheme *runtime.Scheme
	client.Client
	client.Reader
	trace.Tracer
	Logger logr.Logger
}

type tracingStatusClient struct {
	scheme *runtime.Scheme
	client.StatusWriter
	trace.Tracer
	Logger logr.Logger
}

type TracingClient interface {
	client.Client
	trace.Tracer
	// We use this to which calls client.Client Get
	StartTrace(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (context.Context, trace.Span, error)
	EndTrace(ctx context.Context, obj client.Object, opts ...client.PatchOption) error
}

var _ TracingClient = (*tracingClient)(nil)
var _ client.StatusWriter = (*tracingStatusClient)(nil)

// NewTracingClient initializes and returns a new TracingClient
// optional scheme.  If not, it will use client-go scheme
func NewTracingClient(c client.Client, r client.Reader, t trace.Tracer, l logr.Logger, scheme ...*runtime.Scheme) TracingClient {
	tracingScheme := clientgoscheme.Scheme
	if len(scheme) > 0 {
		tracingScheme = scheme[0]
	}

	return &tracingClient{
		scheme: tracingScheme,
		Client: c,
		Reader: r,
		Tracer: t,
		Logger: l,
	}
}

// Create adds tracing and traceID annotation around the original client's Create method
func (tc *tracingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("Create %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Creating object", "object", obj.GetName())
	return tc.Client.Create(ctx, obj, opts...)
}

// Update adds tracing and traceID annotation around the original client's Update method
func (tc *tracingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("Update %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Updating object", "object", obj.GetName())
	return tc.Client.Update(ctx, obj, opts...)
}

// Get adds tracing around the original client's Get method
// IMPORTANT: Caller MUST call `defer span.End()` to end the trace from the calling function
func (tc *tracingClient) StartTrace(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) (context.Context, trace.Span, error) {
	// Create or retrieve the span from the context
	err := tc.Reader.Get(ctx, key, obj, opts...)
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("StartTrace %s %s", obj.GetObjectKind().GroupVersionKind().Kind, key.Name))

	tc.Logger.Info("Getting object", "object", key.Name)
	return trace.ContextWithSpan(ctx, span), span, err
}

// Ends the trace by clearing the traceid from the object
func (tc *tracingClient) EndTrace(ctx context.Context, obj client.Object, opts ...client.PatchOption) error {
	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("EndTrace %s %s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()))
	defer span.End()

	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	// get the current object and ensure that current object has the expected traceid and spanid annotations
	currentObjFromServer := obj.DeepCopyObject().(client.Object)
	tc.Client.Get(ctx, client.ObjectKeyFromObject(obj), currentObjFromServer)

	// compare the traceid and spanid from currentobj to ensure that the traceid and spanid are not changed
	if currentObjFromServer.GetAnnotations()[constants.TraceIDAnnotation] != obj.GetAnnotations()[constants.TraceIDAnnotation] ||
		currentObjFromServer.GetAnnotations()[constants.SpanIDAnnotation] != obj.GetAnnotations()[constants.SpanIDAnnotation] {
		tc.Logger.Info("TraceID or SpanID has changed, skipping patch", "object", obj.GetName())
		return nil
	}

	// Remove the traceid and spanid annotations and create a patch
	original := obj.DeepCopyObject().(client.Object)
	patch := client.MergeFrom(original)

	delete(annotations, constants.TraceIDAnnotation)
	delete(annotations, constants.SpanIDAnnotation)
	obj.SetAnnotations(annotations)

	tc.Logger.Info("Patching object", "object", obj.GetName())
	// Use the Patch function to apply the patch
	return tc.Client.Patch(ctx, obj, patch, opts...)
}

// Get adds tracing around the original client's Get method
func (tc *tracingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	// Create or retrieve the span from the context
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("Get %s %s", kind, key.Name))
	defer span.End()

	tc.Logger.Info("Getting object", "object", key.Name)
	return tc.Client.Get(ctx, key, obj)
}

func (tc *tracingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	ctx, span := startSpanFromContextList(ctx, tc.Logger, tc.Tracer, list, fmt.Sprintf("List %s", list.GetObjectKind().GroupVersionKind().Kind))
	defer span.End()

	tc.Logger.Info("Getting List")
	return tc.Client.List(ctx, list, opts...)
}

// Patch  adds tracing and traceID annotation around the original client's Patch method
func (tc *tracingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("Patch %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	tc.Logger.Info("Patching object", "object", obj.GetName())
	return tc.Client.Patch(ctx, obj, patch, opts...)
}

// Delete adds tracing around the original client's Delete method
func (tc *tracingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("Delete %s %s", kind, obj.GetName()))
	defer span.End()

	tc.Logger.Info("Deleting object", "object", obj.GetName())
	return tc.Client.Delete(ctx, obj, opts...)
}

func (tc *tracingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	gvk, err := apiutil.GVKForObject(obj, tc.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, tc.Logger, tc.Tracer, obj, fmt.Sprintf("DeleteAllOf %s %s", kind, obj.GetName()))
	defer span.End()

	tc.Logger.Info("Deleting all of object", "object", obj.GetName())
	return tc.Client.DeleteAllOf(ctx, obj, opts...)

}

func (tc *tracingClient) Status() client.StatusWriter {
	return &tracingStatusClient{
		scheme:       tc.scheme,
		Logger:       tc.Logger,
		StatusWriter: tc.Client.Status(),
		Tracer:       tc.Tracer,
	}
}

func (ts *tracingStatusClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	gvk, err := apiutil.GVKForObject(obj, ts.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, fmt.Sprintf("StatusUpdate %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	ts.Logger.Info("updating object", "object", obj.GetName())
	return ts.StatusWriter.Update(ctx, obj, opts...)
}

func (ts *tracingStatusClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	gvk, err := apiutil.GVKForObject(obj, ts.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, fmt.Sprintf("StatusPatch %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	return ts.StatusWriter.Patch(ctx, obj, patch, opts...)
}

func (ts *tracingStatusClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	gvk, err := apiutil.GVKForObject(obj, ts.scheme)
	if err != nil {
		return fmt.Errorf("problem getting the scheme: %w", err)
	}

	kind := gvk.GroupKind().Kind

	ctx, span := startSpanFromContext(ctx, ts.Logger, ts.Tracer, obj, fmt.Sprintf("StatusCreate %s %s", kind, obj.GetName()))
	defer span.End()

	addTraceIDAnnotation(ctx, obj)
	return ts.StatusWriter.Create(ctx, obj, subResource, opts...)
}

// startSpanFromContext starts a new span from the context and attaches trace information to the object
func startSpanFromContext(ctx context.Context, logger logr.Logger, tracer trace.Tracer, obj client.Object, operationName string) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		spanContext := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: span.SpanContext().TraceID(),
			SpanID:  span.SpanContext().SpanID(),
		})
		ctx = trace.ContextWithRemoteSpanContext(ctx, spanContext)
		ctx, span = tracer.Start(ctx, operationName)
		return trace.ContextWithSpan(ctx, span), span
	}

	if !span.SpanContext().IsValid() {
		// No valid trace ID in context, check object annotations
		if traceID, ok := obj.GetAnnotations()[constants.TraceIDAnnotation]; ok {
			if traceIDValue, err := trace.TraceIDFromHex(traceID); err == nil {
				spanContext := trace.NewSpanContext(trace.SpanContextConfig{})
				if spanID, ok := obj.GetAnnotations()[constants.SpanIDAnnotation]; ok {
					if spanIDValue, err := trace.SpanIDFromHex(spanID); err == nil {
						spanContext = trace.NewSpanContext(trace.SpanContextConfig{
							TraceID: traceIDValue,
							SpanID:  spanIDValue,
						})
					} else {
						spanContext = trace.NewSpanContext(trace.SpanContextConfig{
							TraceID: traceIDValue,
						})
					}
				}
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
			SpanID:  span.SpanContext().SpanID(),
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
	spanID := span.SpanContext().SpanID().String()
	if spanID != "" {
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(map[string]string{})
		}
		annotations := obj.GetAnnotations()
		annotations[constants.SpanIDAnnotation] = spanID
		obj.SetAnnotations(annotations)
	}
}
