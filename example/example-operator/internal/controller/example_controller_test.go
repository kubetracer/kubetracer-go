package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	v1 "github.com/kubebuilder/kubebuilder-go/example/example-operator/api/v1"
	kubetracer "github.com/kubetracer/kubetracer-go/pkg/client"
	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	v1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	ex := &v1.Example{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-example",
			Namespace: "fake-namespace",
			Labels: map[string]string{
				"configName": "example-configName",
			},
		},
		Spec: v1.ExampleSpec{
			Foo: "bar",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ex).WithStatusSubresource(ex).Build()
	logger := logr.Discard()
	client := kubetracer.NewTracingClient(fakeClient, otel.Tracer("kubetracer"), logger)
	er := &ExampleReconciler{
		Client: client,
	}

	t.Run("test reconcile on example custom resource", func(t *testing.T) {
		req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "fake-example", Namespace: "fake-namespace"}}
		_, err := er.Reconcile(ctx, req)
		if err != nil {
			t.Fatal("failed to reconcile: ", err)
		}
	})

	t.Run("testing if traceId is present on the configmap and the traceId from the context", func(t *testing.T) {
		cm := &corev1.ConfigMap{}
		ctx, err := er.Client.GetWithSpan(ctx, types.NamespacedName{Name: "example-configName", Namespace: "monitoring"}, cm)
		if err != nil {
			t.Fatal("failed to get with span", err)
		}
		traceId := trace.SpanFromContext(ctx).SpanContext().TraceID().String()
		if cm.GetAnnotations()[constants.TraceIDAnnotation] != traceId {
			t.Fatal("traceIds do not match", cm.GetAnnotations()[constants.TraceIDAnnotation], traceId)
		}
	})
}
