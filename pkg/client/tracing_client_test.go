package client

import (
	"context"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func initTracer() trace.Tracer {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)

	return tp.Tracer("kubetracer")
}

func TestNewTracingClient(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	// Check if the client is not nil
	assert.NotNil(t, tracingClient)
}

func TestAutomaticAnnotationManagement(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	// Create a spanId since no GET is being called to initialize the span
	ctx := tracingClient.CreateSpanID(context.Background(), "CreateSpanID")
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	// Create a Pod with an annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// Save the Pod
	err := tracingClient.Create(ctx, pod)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
}

func TestChainReactionTracing(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	// Create an initial Pod
	initialPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "initial-pod",
			Namespace: "default",
		},
	}

	// Create a spanId since no GET is being called to initialize the span
	ctx := tracingClient.CreateSpanID(context.Background(), "CreateSpanID")
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	// Save the initial Pod
	err := tracingClient.Create(ctx, initialPod)
	assert.NoError(t, err)

	// Create a new TracingClient to simulate a fresh client
	newK8sClient := fake.NewFakeClient(initialPod)
	newTracingClient := NewTracingClient(newK8sClient, tracer, logger)

	// Retrieve the initial Pod to get the trace ID
	retrievedInitialPod := &corev1.Pod{}
	err = newTracingClient.Get(ctx, client.ObjectKey{Name: "initial-pod", Namespace: "default"}, retrievedInitialPod)
	assert.NoError(t, err)

	// Extract the trace ID from the retrieved initial pod annotations
	savedtraceID := retrievedInitialPod.Annotations[constants.TraceIDAnnotation]
	assert.Equal(t, traceID, savedtraceID)
}

func TestUpdateWithTracing(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewClientBuilder().WithObjects(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-test-pod",
			Namespace: "default",
		},
	}).Build()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, err := tracingClient.GetWithSpan(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	assert.NoError(t, err)
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()

	// Create a Pod with an annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// Save the Pod
	err = tracingClient.Create(ctx, pod)
	assert.NoError(t, err)

	// Update the Pod
	pod.Labels = map[string]string{"updated": "true"}
	err = tracingClient.Update(ctx, pod)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
}
