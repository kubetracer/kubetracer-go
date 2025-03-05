package client

import (
	"context"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
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

	// Create a Pod with an annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"kubetracer.io/parent-span-id": "12345",
			},
		},
	}

	// Save the Pod
	err := tracingClient.Create(context.Background(), pod)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(context.Background(), client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, "12345", retrievedPod.Annotations["kubetracer.io/parent-span-id"])
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

	// Save the initial Pod
	err := tracingClient.Create(context.Background(), initialPod)
	assert.NoError(t, err)

	// Create a child Pod within the same trace context
	childPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child-pod",
			Namespace: "default",
		},
	}

	// Update or create the child Pod
	err = tracingClient.Create(context.Background(), childPod)
	assert.NoError(t, err)

	// Check if the child Pod has the trace context annotation
	retrievedChildPod := &corev1.Pod{}
	err = tracingClient.Get(context.Background(), client.ObjectKey{Name: "child-pod", Namespace: "default"}, retrievedChildPod)
	assert.NoError(t, err)
	assert.NotEmpty(t, retrievedChildPod.Annotations["kubetracer.io/parent-span-id"])
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

	// Create a Pod with an annotation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"kubetracer.io/trace-id": "12345",
			},
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
	assert.Equal(t, "12345", retrievedPod.Annotations["kubetracer.io/trace-id"])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
}
