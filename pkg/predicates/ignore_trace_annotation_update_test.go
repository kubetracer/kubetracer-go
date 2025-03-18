package predicates

import (
	"testing"

	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIgnoreTraceAnnotationUpdatePredicate(t *testing.T) {
	pred := IgnoreTraceAnnotationUpdatePredicate{}

	// Test case: Only trace ID and resource version annotations changed
	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.TraceIDAnnotation: "old-trace-id",
				constants.SpanIDAnnotation:  "old-span-id",
				"key1":                      "value1",
			},
			ResourceVersion: "old-resource-version",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}

	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constants.TraceIDAnnotation: "new-trace-id",
				constants.SpanIDAnnotation:  "new-span-id",
				"key1":                      "value1",
			},
			ResourceVersion: "new-resource-version",
		},
		Spec: corev1.PodSpec{
			Containers: nil,
		},
	}

	updateEvent := event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: newPod,
	}

	// Only trace ID and resource version changed, should return false
	result := pred.Update(updateEvent)
	assert.False(t, result, "Expected update to be ignored when only trace ID and resource version annotations change")

	// Test case: Another annotation changed
	newPod.Annotations["key1"] = "new-value1"
	updateEvent = event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: newPod,
	}

	// Another annotation changed, should return true
	result = pred.Update(updateEvent)
	assert.True(t, result, "Expected update to be processed when other annotations change")

	// Test case: Spec changed
	oldPod.Spec.Containers = []corev1.Container{
		{
			Name:  "nginx",
			Image: "nginx:1.14.2",
		},
	}
	newPod.Spec.Containers = []corev1.Container{
		{
			Name:  "nginx",
			Image: "nginx:1.15.0",
		},
	}
	updateEvent = event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: newPod,
	}

	// Spec changed, should return true
	result = pred.Update(updateEvent)
	assert.True(t, result, "Expected update to be processed when spec changes")

	// Test case: Status changed
	oldPod.Status.Phase = corev1.PodPending
	newPod.Status.Phase = corev1.PodRunning
	updateEvent = event.UpdateEvent{
		ObjectOld: oldPod,
		ObjectNew: newPod,
	}

	// Status changed, should return true
	result = pred.Update(updateEvent)
	assert.True(t, result, "Expected update to be processed when status changes")
}
