package main

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/kubetracer/kubetracer-go/client"
	"go.opentelemetry.io/otel"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type MyController struct {
	Client *client.TracingClient
	Logger logr.Logger
}

func (r *MyController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// Get the resource being reconciled
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Perform reconcile logic here
	// Example: Add a label to the pod
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["kubetracer"] = "enabled"

	// Update the pod
	if err := r.Client.Update(ctx, pod); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func add(mgr manager.Manager, logger logr.Logger) error {
	// Setup the controller
	c, err := controller.New("my-controller", mgr, controller.Options{
		Reconciler: &MyController{
			Client: client.NewTracingClient(mgr.GetClient(), otel.Tracer("kubetracer"), logger),
			Logger: logger,
		},
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// Set up the logger
	logger := zap.New(zap.UseDevMode(true))

	// Create a new manager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		logger.Error(err, "Unable to set up overall controller manager")
		os.Exit(1)
	}

	// Add the controller to the manager
	if err := add(mgr, logger); err != nil {
		logger.Error(err, "Unable to add controller to manager")
		os.Exit(1)
	}

	// Start the manager
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		logger.Error(err, "Unable to start manager")
		os.Exit(1)
	}
}
