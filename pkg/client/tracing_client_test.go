package client

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
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
	k8sClient := fake.NewClientBuilder().WithObjects(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-test-pod",
			Namespace: "default",
		},
	}).Build()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := logr.Discard()
	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	// Check if the client is not nil
	assert.NotNil(t, tracingClient)
}

func TestAutomaticAnnotationManagement(t *testing.T) {
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
	logger := logr.Discard()
	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()

	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

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

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.NotEqual(t, spanID, retrievedPod.Annotations[constants.SpanIDAnnotation])
	assert.Equal(t, len(spanID), len(retrievedPod.Annotations[constants.SpanIDAnnotation]))
}

func TestPassingTraceIdInNamespacedName(t *testing.T) {
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
	logger := logr.Discard()
	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()

	key := client.ObjectKey{Name: "f620f5cad0af940c294f980c5366a6a1;45f359cdc1c8ab06;pre-test-pod", Namespace: "default"}

	// Create a spanId since no GET is being called to initialize the span
	_, span, err := tracingClient.StartTrace(ctx, key, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()

	assert.Equal(t, "f620f5cad0af940c294f980c5366a6a1", traceID)
}

func TestChainReactionTracing(t *testing.T) {
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
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	// Create an initial Pod
	initialPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "initial-pod",
			Namespace: "default",
		},
	}

	ctx := context.Background()

	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Save the initial Pod
	err = tracingClient.Create(ctx, initialPod)
	assert.NoError(t, err)

	// Create a new TracingClient to simulate a fresh client
	newK8sClient := fake.NewClientBuilder().WithObjects(initialPod).Build()
	newTracingClient := NewTracingClient(newK8sClient, newK8sClient, tracer, logger)

	// Retrieve the initial Pod to get the trace ID
	retrievedInitialPod := &corev1.Pod{}
	err = newTracingClient.Get(ctx, client.ObjectKey{Name: "initial-pod", Namespace: "default"}, retrievedInitialPod)
	assert.NoError(t, err)

	// Extract the trace ID from the retrieved initial pod annotations
	savedtraceID := retrievedInitialPod.Annotations[constants.TraceIDAnnotation]
	savedSpanID := retrievedInitialPod.Annotations[constants.SpanIDAnnotation]
	assert.Equal(t, traceID, savedtraceID)
	assert.NotEqual(t, spanID, savedSpanID)

	t.Run("", func(t *testing.T) {
		patchPod := client.MergeFrom(retrievedInitialPod.DeepCopy())
		retrievedInitialPod.Status.Phase = corev1.PodRunning
		err := newTracingClient.Status().Patch(ctx, retrievedInitialPod, patchPod)
		assert.NoError(t, err)
		assert.Equal(t, retrievedInitialPod.Status.Phase, corev1.PodRunning)
		retrievedPatchedPod := &corev1.Pod{}
		err = newTracingClient.Get(ctx, client.ObjectKey{Name: "initial-pod", Namespace: "default"}, retrievedPatchedPod)
		assert.NoError(t, err)
		assert.Equal(t, savedtraceID, retrievedPatchedPod.Annotations[constants.TraceIDAnnotation])
		//Annotations will not be patched with Status.Patch
		assert.Equal(t, savedSpanID, retrievedPatchedPod.Annotations[constants.SpanIDAnnotation])
	})
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
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

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
	assert.NotEqual(t, spanID, retrievedPod.Annotations[constants.SpanIDAnnotation])
	assert.Equal(t, len(spanID), len(retrievedPod.Annotations[constants.SpanIDAnnotation]))

	// Test status udpate with tracing
	t.Run("update status with tracing", func(t *testing.T) {
		pod.Status.Phase = corev1.PodRunning
		err = tracingClient.Status().Update(ctx, retrievedPod)
		assert.NoError(t, err)
		assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	})
}

func TestPatchWithTracing(t *testing.T) {
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
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

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

	// Patch the Pod
	podPatch := client.MergeFrom(pod.DeepCopy())
	pod.Labels = map[string]string{"updated": "true"}
	err = tracingClient.Patch(ctx, pod, podPatch)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
	assert.NotEqual(t, spanID, retrievedPod.Annotations[constants.SpanIDAnnotation])
	assert.Equal(t, len(spanID), len(retrievedPod.Annotations[constants.SpanIDAnnotation]))

	t.Run("status create with tracing", func(t *testing.T) {
		err := tracingClient.Status().Create(ctx, retrievedPod, retrievedPod)
		// fakeClient does not support Create for subresoruce Client
		// https://github.com/kubernetes-sigs/controller-runtime/blob/v0.20.3/pkg/client/fake/client.go#L1227
		assert.Error(t, err)
	})
}

func TestEndTrace(t *testing.T) {
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
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

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

	// Patch the Pod
	podPatch := client.MergeFrom(pod.DeepCopy())
	pod.Labels = map[string]string{"updated": "true"}
	err = tracingClient.Patch(ctx, pod, podPatch)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
	assert.NotEqual(t, spanID, retrievedPod.Annotations[constants.SpanIDAnnotation])
	assert.Equal(t, len(spanID), len(retrievedPod.Annotations[constants.SpanIDAnnotation]))

	// Test EndTrace
	_, err = tracingClient.EndTrace(ctx, retrievedPod)
	assert.NoError(t, err)
	finalPod := &corev1.Pod{}
	// Get the pod with default kubernetes client to ensure that there is no traceID and spanID
	err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, finalPod)
	assert.NoError(t, err)
	assert.Empty(t, finalPod.Annotations[constants.TraceIDAnnotation])
	assert.Empty(t, finalPod.Annotations[constants.SpanIDAnnotation])
}

func TestEndTraceChangedAnnotation(t *testing.T) {
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
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()

	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

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

	// Patch the Pod
	podPatch := client.MergeFrom(pod.DeepCopy())
	pod.Labels = map[string]string{"updated": "true"}
	err = tracingClient.Patch(ctx, pod, podPatch)
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, retrievedPod)
	assert.NoError(t, err)
	assert.Equal(t, traceID, retrievedPod.Annotations[constants.TraceIDAnnotation])
	assert.Equal(t, "true", retrievedPod.Labels["updated"])
	assert.NotEqual(t, spanID, retrievedPod.Annotations[constants.SpanIDAnnotation])
	assert.Equal(t, len(spanID), len(retrievedPod.Annotations[constants.SpanIDAnnotation]))

	// Initialize the TracingClient
	tracingClientNew := NewTracingClient(k8sClient, k8sClient, tracer, logger)
	ctxNew := context.Background()
	ctxNew, spanNew, errNew := tracingClientNew.StartTrace(ctxNew, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer spanNew.End()
	assert.NoError(t, errNew)
	traceIDNew := spanNew.SpanContext().TraceID().String()
	retrievedPodClone := retrievedPod.DeepCopy()
	retrievedPodClone.Status.HostIP = "11.12.13.14"
	tracingClientNew.Update(ctxNew, retrievedPodClone)

	// Test EndTrace and ensure that it did not remove the traceID since it was updated by a different client
	_, err = tracingClient.EndTrace(ctx, retrievedPod)
	assert.NoError(t, err)
	finalPod := &corev1.Pod{}
	// Get the pod with default kubernetes client to ensure that there is no traceID and spanID
	err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-pod", Namespace: "default"}, finalPod)
	assert.NoError(t, err)
	assert.Equal(t, traceIDNew, finalPod.Annotations[constants.TraceIDAnnotation])
	assert.NotEmpty(t, finalPod.Annotations[constants.SpanIDAnnotation])
}

func TestListWithTracing(t *testing.T) {
	// Create a fake Kubernetes client
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-test-pod",
			Namespace: "default",
		},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(pod).Build()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()
	assert.NoError(t, err)

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.PodList{}
	err = tracingClient.List(ctx, retrievedPod)
	assert.NoError(t, err)
	span = trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	assert.NotEmpty(t, traceID)

}

func TestDeleteWithTracing(t *testing.T) {
	// Create a fake Kubernetes client
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-test-pod",
			Namespace: "default",
		},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(pod).Build()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()
	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()

	// Retrieve the Pod and check the annotation
	err = tracingClient.Delete(ctx, pod)
	assert.NoError(t, err)
	span = trace.SpanFromContext(ctx)
	traceID = span.SpanContext().TraceID().String()
	assert.NotEmpty(t, traceID)
}

func TestDeleteAllOfWithTracing(t *testing.T) {
	// Create a fake Kubernetes client
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pre-test-pod",
			Namespace: "default",
		},
	}
	k8sClient := fake.NewClientBuilder().WithObjects(pod).Build()

	// Create a real tracer
	tracer := initTracer()

	// Create a logger
	logger := logr.Discard()

	// Initialize the TracingClient
	tracingClient := NewTracingClient(k8sClient, k8sClient, tracer, logger)

	ctx := context.Background()
	// Create a spanId since no GET is being called to initialize the span
	ctx, span, err := tracingClient.StartTrace(ctx, client.ObjectKey{Name: "pre-test-pod", Namespace: "default"}, &corev1.Pod{})
	defer span.End()
	assert.NoError(t, err)
	traceID := span.SpanContext().TraceID().String()

	// Retrieve the Pod and check the annotation
	retrievedPod := &corev1.Pod{}
	err = tracingClient.DeleteAllOf(ctx, retrievedPod)
	assert.NoError(t, err)
	span = trace.SpanFromContext(ctx)
	traceID = span.SpanContext().TraceID().String()
	assert.NotEmpty(t, traceID)

}
