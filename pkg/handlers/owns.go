/*
Original source: https://github.com/kubernetes-sigs/controller-runtime/blob/v0.20.3/pkg/handler/enqueue_owner.go
Has been modified to suit the project's needs.
*/

package handler

import (
	"context"
	"fmt"

	"github.com/kubetracer/kubetracer-go/pkg/constants"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type empty struct{}

type requestWithTraceID struct {
	NamespacedName reconcile.Request
	TraceID        string
	SpanID         string
	SenderName     string
	SenderKind     string
	EventKind      string
}

var _ handler.EventHandler = &enqueueRequestForOwner[client.Object]{}

// OwnerOption modifies an EnqueueRequestForOwner EventHandler.
type OwnerOption func(e enqueueRequestForOwnerInterface)

// EnqueueRequestForOwner enqueues Requests for the Owners of an object.  E.g. the object that created
// the object that was the source of the Event.
//
// If a ReplicaSet creates Pods, users may reconcile the ReplicaSet in response to Pod Events using:
//
// - a source.Kind Source with Type of Pod.
//
// - a handler.enqueueRequestForOwner EventHandler with an OwnerType of ReplicaSet and OnlyControllerOwner set to true.
func EnqueueRequestForOwner(scheme *runtime.Scheme, mapper meta.RESTMapper, ownerType client.Object, opts ...OwnerOption) handler.EventHandler {
	return TypedEnqueueRequestForOwner[client.Object](scheme, mapper, ownerType, opts...)
}

// TypedEnqueueRequestForOwner enqueues Requests for the Owners of an object.  E.g. the object that created
// the object that was the source of the Event.
//
// If a ReplicaSet creates Pods, users may reconcile the ReplicaSet in response to Pod Events using:
//
// - a source.Kind Source with Type of Pod.
//
// - a handler.typedEnqueueRequestForOwner EventHandler with an OwnerType of ReplicaSet and OnlyControllerOwner set to true.
//
// TypedEnqueueRequestForOwner is experimental and subject to future change.
func TypedEnqueueRequestForOwner[object client.Object](scheme *runtime.Scheme, mapper meta.RESTMapper, ownerType client.Object, opts ...OwnerOption) handler.TypedEventHandler[object, reconcile.Request] {
	e := &enqueueRequestForOwner[object]{
		ownerType: ownerType,
		mapper:    mapper,
		scheme:    scheme,
	}
	if err := e.parseOwnerTypeGroupKind(scheme); err != nil {
		panic(err)
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// OnlyControllerOwner if provided will only look at the first OwnerReference with Controller: true.
func OnlyControllerOwner() OwnerOption {
	return func(e enqueueRequestForOwnerInterface) {
		e.setIsController(true)
	}
}

type enqueueRequestForOwnerInterface interface {
	setIsController(bool)
}

type enqueueRequestForOwner[object client.Object] struct {
	// ownerType is the type of the Owner object to look for in OwnerReferences.  Only Group and Kind are compared.
	ownerType runtime.Object

	// isController if set will only look at the first OwnerReference with Controller: true.
	isController bool

	// groupKind is the cached Group and Kind from OwnerType
	groupKind schema.GroupKind

	// mapper maps GroupVersionKinds to Resources
	mapper meta.RESTMapper

	// scheme is used to get the GroupVersionKind of the object
	scheme *runtime.Scheme
}

func (e *enqueueRequestForOwner[object]) setIsController(isController bool) {
	e.isController = isController
}

// Create implements EventHandler.
func (e *enqueueRequestForOwner[object]) Create(ctx context.Context, evt event.TypedCreateEvent[object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	reqs := map[requestWithTraceID]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs, "new")
	res := requestWithTraceIDToRequest(reqs)
	for req := range res {
		q.Add(req)
	}
}

// Update implements EventHandler.
func (e *enqueueRequestForOwner[object]) Update(ctx context.Context, evt event.TypedUpdateEvent[object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	reqs := map[requestWithTraceID]empty{}
	e.getOwnerReconcileRequest(evt.ObjectOld, reqs, "old")
	e.getOwnerReconcileRequest(evt.ObjectNew, reqs, "new")
	res := requestWithTraceIDToRequest(reqs)
	for req := range res {
		q.Add(req)
	}
}

// Delete implements EventHandler.
func (e *enqueueRequestForOwner[object]) Delete(ctx context.Context, evt event.TypedDeleteEvent[object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	reqs := map[requestWithTraceID]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs, "new")
	res := requestWithTraceIDToRequest(reqs)
	for req := range res {
		q.Add(req)
	}
}

// Generic implements EventHandler.
func (e *enqueueRequestForOwner[object]) Generic(ctx context.Context, evt event.TypedGenericEvent[object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	reqs := map[requestWithTraceID]empty{}
	e.getOwnerReconcileRequest(evt.Object, reqs, "new")
	res := requestWithTraceIDToRequest(reqs)
	for req := range res {
		q.Add(req)
	}
}

// parseOwnerTypeGroupKind parses the OwnerType into a Group and Kind and caches the result.  Returns false
// if the OwnerType could not be parsed using the scheme.
func (e *enqueueRequestForOwner[object]) parseOwnerTypeGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(e.ownerType)
	if err != nil {
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		err := fmt.Errorf("expected exactly 1 kind for OwnerType %T, but found %s kinds", e.ownerType, kinds)
		return err
	}
	// Cache the Group and Kind for the OwnerType
	e.groupKind = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}

// getOwnerReconcileRequest looks at object and builds a map of reconcile.Request to reconcile
// owners of object that match e.OwnerType.
func (e *enqueueRequestForOwner[object]) getOwnerReconcileRequest(obj metav1.Object, result map[requestWithTraceID]empty, eventKind string) {
	// Iterate through the OwnerReferences looking for a match on Group and Kind against what was requested
	// by the user
	for _, ref := range e.getOwnersReferences(obj) {
		// Parse the Group out of the OwnerReference to compare it to what was parsed out of the requested OwnerType
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			// log.Error(err, "Could not parse OwnerReference APIVersion",
			// 	"api version", ref.APIVersion)
			return
		}

		runtimeObj, _ := obj.(runtime.Object)
		gvk, err := apiutil.GVKForObject(runtimeObj, e.scheme)
		kind := ""
		if err != nil {
			// log.Error(err, "Could not retrieve GVK for object", "object", obj)
			return
		} else {
			kind = gvk.GroupKind().Kind
		}

		// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind specified by the user.
		// If the two match, create a Request for the objected referred to by
		// the OwnerReference.  Use the Name from the OwnerReference and the Namespace from the
		// object in the event.
		if ref.Kind == e.groupKind.Kind && refGV.Group == e.groupKind.Group {
			// Match found - add a Request for the object referred to in the OwnerReference
			request := requestWithTraceID{
				NamespacedName: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: ref.Name,
					},
				},
			}

			// if owner is not namespaced then we should not set the namespace
			mapping, err := e.mapper.RESTMapping(e.groupKind, refGV.Version)
			if err != nil {
				// log.Error(err, "Could not retrieve rest mapping", "kind", e.groupKind)
				return
			}
			if mapping.Scope.Name() != meta.RESTScopeNameRoot {
				request.NamespacedName.Namespace = obj.GetNamespace()
			}

			traceId := obj.GetAnnotations()[constants.TraceIDAnnotation]
			spanId := obj.GetAnnotations()[constants.SpanIDAnnotation]
			senderName := obj.GetName()
			senderKind := kind

			if traceId != "" && spanId != "" {
				request.TraceID = traceId
				request.SpanID = spanId
			}

			request.EventKind = eventKind
			request.SenderName = senderName
			request.SenderKind = senderKind

			result[request] = empty{}
		}
	}
}

// Converts the reqeustWithTraceID map to a request map and uses the EmbedTraceIDInNamespacedName function to set the name
func requestWithTraceIDToRequest(requests map[requestWithTraceID]empty) map[reconcile.Request]empty {
	result := map[reconcile.Request]empty{}

	// todo
	// dedupe based on name, if the name is the same, choose the one that is "new" eventKind
	// todo

	for req := range requests {
		key := reconcile.Request{}

		var name string

		if req.TraceID != "" && req.SpanID != "" {
			name = fmt.Sprintf("%s;%s;%s;%s;%s", req.TraceID, req.SpanID, req.SenderKind, req.SenderName, req.NamespacedName.Name)
		} else {
			name = req.NamespacedName.Name
		}

		// Embed the trace ID in the NamespacedName
		namespace := req.NamespacedName.Namespace
		key.NamespacedName = types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}

		result[key] = empty{}
	}
	return result
}

// getOwnersReferences returns the OwnerReferences for an object as specified by the enqueueRequestForOwner
// - if IsController is true: only take the Controller OwnerReference (if found)
// - if IsController is false: take all OwnerReferences.
func (e *enqueueRequestForOwner[object]) getOwnersReferences(obj metav1.Object) []metav1.OwnerReference {
	if obj == nil {
		return nil
	}

	// If not filtered as Controller only, then use all the OwnerReferences
	if !e.isController {
		return obj.GetOwnerReferences()
	}
	// If filtered to a Controller, only take the Controller OwnerReference
	if ownerRef := metav1.GetControllerOf(obj); ownerRef != nil {
		return []metav1.OwnerReference{*ownerRef}
	}
	// No Controller OwnerReference found
	return nil
}
