package controller

import (
	"context"

	kubetracer "github.com/kubetracer/kubetracer-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ConfigMapReconciler reconciles a ConfigMap object
type ConfigMapReconciler struct {
	Client kubetracer.TracingClient
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;create;update;patch

// Reconcile function to reconcile ConfigMap and create a corresponding Secret
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ConfigMap instance
	configMap := &corev1.ConfigMap{}
	ctx, err := r.Client.GetWithSpan(ctx, req.NamespacedName, configMap)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Define the Secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMap.Name + "-secret",
			Namespace: configMap.Namespace,
		},
		StringData: configMap.Data,
	}

	// Create or Update the Secret
	if err := r.Client.Create(ctx, secret); err != nil {
		// Update the Secret if it already exists
		existingSecret := &corev1.Secret{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, existingSecret); err != nil {
			log.Error(err, "unable to fetch Secret")
			return ctrl.Result{}, err
		}
		existingSecret.StringData = secret.StringData
		if err := r.Client.Update(ctx, existingSecret); err != nil {
			log.Error(err, "unable to update Secret")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}
