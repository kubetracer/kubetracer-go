/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	kubetracer "github.com/kubetracer/kubetracer-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	examplev1 "github.com/kubebuilder/kubebuilder-go/example/example-operator/api/v1"
)

// ExampleReconciler reconciles a Example object
type ExampleReconciler struct {
	Client kubetracer.TracingClient
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=examples,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Example object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *ExampleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	example := &examplev1.Example{}
	ctx, err := r.Client.GetWithSpan(ctx, req.NamespacedName, example)
	if err != nil {
		return ctrl.Result{}, err
	}

	// create configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      example.GetLabels()["configName"],
			Namespace: "monitoring",
		},
		Data: map[string]string{
			"example-key": "example-value",
		},
	}
	if err := r.Client.Create(ctx, configMap); err != nil {
		return ctrl.Result{}, err
	}

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&examplev1.Example{}).
		Named("example").
		Complete(r)
}
