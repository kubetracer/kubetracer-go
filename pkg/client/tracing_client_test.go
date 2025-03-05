package client

import (
	"context"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewTracingClient(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a noop tracer
	tracer := noop.NewTracerProvider().Tracer("kubetracer")

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

	// Create a noop tracer
	tracer := noop.NewTracerProvider().Tracer("kubetracer")

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	// Create a spanId since no GET is being called to initialize the span
	ctx := tracingClient.CreateSpanID(context.Background(), "CreateSpanID")

	span := trace.SpanFromContext(ctx)

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
	err = tracingClient.Get(context.Background(), client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, span.SpanContext().TraceID().String(), retrievedPod.Annotations[constants.TraceIDAnnotation])
}

func TestChainReactionTracing(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a noop tracer
	tracer := noop.NewTracerProvider().Tracer("kubetracer")

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

	// Save the initial Pod
	err := tracingClient.Create(ctx, initialPod)
	assert.NoError(t, err)

	// Create a new TracingClient to simulate a fresh client
	newK8sClient := fake.NewFakeClient(initialPod)
	newTracingClient := NewTracingClient(newK8sClient, tracer, logger)

	// Retrieve the initial Pod to get the trace ID
	retrievedInitialPod := &corev1.Pod{}
	err = newTracingClient.Get(context.Background(), client.ObjectKey{Name: "initial-pod", Namespace: "default"}, retrievedInitialPod)
	assert.NoError(t, err)

	// Extract the trace ID from the retrieved initial pod annotations
	traceID := retrievedInitialPod.Annotations[constants.TraceIDAnnotation]

	assert.Equal(t, span.SpanContext().TraceID().String(), traceID)
}

func TestUpdateWithTracing(t *testing.T) {
	// Create a fake Kubernetes client
	k8sClient := fake.NewFakeClient()

	// Create a noop tracer
	tracer := noop.NewTracerProvider().Tracer("kubetracer")

	// Create a logger
	logger := funcr.New(func(prefix, args string) {
		println(prefix + ": " + args)
	}, funcr.Options{})

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, tracer, logger)

	// Create a spanId since no GET is being called to initialize the span
	ctx := tracingClient.CreateSpanID(context.Background(), "CreateSpanID")
	span := trace.SpanFromContext(ctx)

	// Create a Pod with an annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	// Save the Pod
	err := tracingClient.Create(context.Background(), pod)
	assert.NoError(t, err)

	// Update the Pod
	pod.Labels = map[string]string{"updated": "true"}
	err = tracingClient.Update(context.Background(), pod)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(context.Background(), client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, span.SpanContext().TraceID().String(), retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
}
