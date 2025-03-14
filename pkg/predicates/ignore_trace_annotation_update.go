package predicates

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
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

	// Perform deep copies to avoid concurrent map writes issues
	oldObjCopy := e.ObjectOld.DeepCopyObject().(runtime.Object)
	newObjCopy := e.ObjectNew.DeepCopyObject().(runtime.Object)

	// Remove the traceid and resource version from both old and new objects
	annotationsOld, err := meta.NewAccessor().Annotations(oldObjCopy)
	if err == nil {
		delete(annotationsOld, constants.TraceIDAnnotation)
		delete(annotationsOld, constants.SpanIDAnnotation)
	}
	meta.NewAccessor().SetAnnotations(oldObjCopy, annotationsOld)
	meta.NewAccessor().SetResourceVersion(oldObjCopy, "")

	annotationsNew, err := meta.NewAccessor().Annotations(newObjCopy)
	if err == nil {
		delete(annotationsNew, constants.TraceIDAnnotation)
		delete(annotationsNew, constants.SpanIDAnnotation)
	}
	meta.NewAccessor().SetAnnotations(newObjCopy, annotationsNew)
	meta.NewAccessor().SetResourceVersion(newObjCopy, "")

	// Check if the spec or status have changed
	if !reflect.DeepEqual(oldObjCopy, newObjCopy) {
		return true
	}

	// If we reach here, the only change was the trace ID annotation
	return false
}
