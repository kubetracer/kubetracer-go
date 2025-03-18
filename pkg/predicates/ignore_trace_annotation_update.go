package predicates

import (
	"github.com/google/go-cmp/cmp"
	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IgnoreTraceAnnotationUpdatePredicate implements a predicate that ignores updates where only the
// trace ID and span ID annotations, or resource version changes.
type IgnoreTraceAnnotationUpdatePredicate struct {
	predicate.Funcs
}

// Update implements the update event check for the predicate.
func (IgnoreTraceAnnotationUpdatePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return true
	}

	oldAnnotations := e.ObjectOld.GetAnnotations()
	newAnnotations := e.ObjectNew.GetAnnotations()

	traceIDChanged := oldAnnotations[constants.TraceIDAnnotation] != newAnnotations[constants.TraceIDAnnotation]
	spanIDChanged := oldAnnotations[constants.SpanIDAnnotation] != newAnnotations[constants.SpanIDAnnotation]
	resourceVersionChanged := e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
	otherAnnotationsChanged := !equalExcept(oldAnnotations, newAnnotations, constants.TraceIDAnnotation, constants.SpanIDAnnotation)

	// Check if the spec or status fields have changed
	specChanged := hasSpecChanged(e.ObjectOld, e.ObjectNew)
	statusChanged := hasStatusChanged(e.ObjectOld, e.ObjectNew)

	// If only trace ID, span ID, or resource version changed, and no other annotations, spec or status changed, ignore the update
	if (traceIDChanged || spanIDChanged || resourceVersionChanged) && !otherAnnotationsChanged && !specChanged && !statusChanged {
		return false
	}

	// Otherwise, indicate the update should be processed
	return true
}

// Helper functions to check if spec or status have changed:
func hasSpecChanged(oldObj, newObj runtime.Object) bool {
	oldUnstructured := objToUnstructured(oldObj)
	newUnstructured := objToUnstructured(newObj)
	oldSpec, foundOld, _ := unstructuredNestedFieldCopy(oldUnstructured, "spec")
	newSpec, foundNew, _ := unstructuredNestedFieldCopy(newUnstructured, "spec")
	return foundOld != foundNew || !cmp.Equal(oldSpec, newSpec)
}

func hasStatusChanged(oldObj, newObj runtime.Object) bool {
	oldUnstructured := objToUnstructured(oldObj)
	newUnstructured := objToUnstructured(newObj)
	oldStatus, foundOld, _ := unstructuredNestedFieldCopy(oldUnstructured, "status")
	newStatus, foundNew, _ := unstructuredNestedFieldCopy(newUnstructured, "status")
	return foundOld != foundNew || !cmp.Equal(oldStatus, newStatus)
}

// Checks if two maps are equal, ignoring certain keys
func equalExcept(a, b map[string]string, keysToIgnore ...string) bool {
	ignored := map[string]struct{}{}
	for _, key := range keysToIgnore {
		ignored[key] = struct{}{}
	}

	for key, aValue := range a {
		if _, isIgnored := ignored[key]; !isIgnored {
			if bValue, exists := b[key]; !exists || aValue != bValue {
				return false
			}
		}
	}

	for key := range b {
		if _, exists := a[key]; !exists {
			if _, isIgnored := ignored[key]; !isIgnored {
				return false
			}
		}
	}

	return true
}

func objToUnstructured(obj runtime.Object) map[string]interface{} {
	unstructuredMap, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	return unstructuredMap
}

func unstructuredNestedFieldCopy(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	val, found, err := unstructured.NestedFieldCopy(obj, fields...)
	if !found || err != nil {
		return nil, false, err
	}
	return val, true, nil
}
