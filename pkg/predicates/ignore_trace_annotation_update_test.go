package predicates_test

import (
	"testing"

	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"github.com/kubetracer/kubetracer-go/pkg/predicates"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIgnoreTraceAnnotationUpdatePredicate(t *testing.T) {
	pred := predicates.IgnoreTraceAnnotationUpdatePredicate{}

	t.Run("only trace ID and resource version annotations changed", func(t *testing.T) {
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

		result := pred.Update(updateEvent)
		assert.False(t, result, "Expected update to be ignored when only trace ID and resource version annotations change")
	})

	t.Run("another annotation changed", func(t *testing.T) {
		oldPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.TraceIDAnnotation: "old-trace-id",
					constants.SpanIDAnnotation:  "old-span-id",
					"key1":                      "value1",
				},
				ResourceVersion: "old-resource-version",
			},
		}

		newPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					constants.TraceIDAnnotation: "old-trace-id",
					constants.SpanIDAnnotation:  "old-span-id",
					"key1":                      "new-value",
				},
				ResourceVersion: "new-resource-version",
			},
		}

		updateEvent := event.UpdateEvent{
			ObjectOld: oldPod,
			ObjectNew: newPod,
		}

		result := pred.Update(updateEvent)
		assert.True(t, result, "Expected update to be processed when other annotations change")
	})

	t.Run("spec changed", func(t *testing.T) {
		oldPod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.14.2",
					},
				},
			},
		}

		newPod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.15.0",
					},
				},
			},
		}

		updateEvent := event.UpdateEvent{
			ObjectOld: oldPod,
			ObjectNew: newPod,
		}

		result := pred.Update(updateEvent)
		assert.True(t, result, "Expected update to be processed when spec changes")
	})

	t.Run("status changed", func(t *testing.T) {
		oldPod := &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}

		newPod := &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}

		updateEvent := event.UpdateEvent{
			ObjectOld: oldPod,
			ObjectNew: newPod,
		}

		result := pred.Update(updateEvent)
		assert.True(t, result, "Expected update to be processed when status changes")
	})
}
