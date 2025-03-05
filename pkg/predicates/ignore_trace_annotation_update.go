package predicates

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	constants "github.com/kubetracer/kubetracer-go/pkg/constants"
)

// IgnoreTraceAnnotationUpdatePredicate implements a predicate that ignores updates where only the trace ID annotation changes.
type IgnoreTraceAnnotationUpdatePredicate struct {
	predicate.Funcs
}

// Update implements the update event check for the predicate.
func (IgnoreTraceAnnotationUpdatePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return true
	}

	// remove the traceid and resource version from both old and new objects
	delete(e.ObjectOld.GetAnnotations(), constants.TraceIDAnnotation)
	delete(e.ObjectOld.GetAnnotations(), constants.ResourceVersionKey)
	delete(e.ObjectNew.GetAnnotations(), constants.TraceIDAnnotation)
	delete(e.ObjectNew.GetAnnotations(), constants.ResourceVersionKey)

	// Check if the spec or status have changed
	if !reflect.DeepEqual(e.ObjectOld.DeepCopyObject(), e.ObjectNew.DeepCopyObject()) {
		return true
	}

	// If we reach here, the only change was the trace ID annotation
	return false
}
