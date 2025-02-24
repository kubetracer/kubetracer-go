# kubetracer-go
Golang library Implementation for Kubetracer


## kubetracer

kubetracer is a lightweight library designed to facilitate tracing in Kubernetes controller-based projects. It simplifies the process of automatically adding and reading annotations for parent trace IDs on Kubernetes resources, enabling developers to trace and debug complex interactions between resources and controllers efficiently.

## Features

- Automatic Annotation Management: Automatically add and read annotations for parent trace IDs on Kubernetes resources.
- Integration with OpenTelemetry: Leverage OpenTelemetry for standardized tracing and observability.
- Chain Reaction Tracing: Track and visualize chain reactions triggered by controller reconciles.
- Lightweight and Easy to Use: A minimalistic library that integrates seamlessly with your existing controller code.

## Installation

To install kubetracer, add it as a dependency to your Go project:

```bash
go get github.com/kubetracer/kubetracer
```

## Usage

### Integrating kubetracer in Your Controller

To integrate kubetracer into your Kubernetes controller, follow these steps:

1. Import kubetracer:

```golang
import (
    "context"
    "github.com/go-logr/logr"
    "github.com/kubetracer/kubetracer"
    "github.com/kubetracer/kubetracer/predicates"
    "go.opentelemetry.io/otel"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/reconcile"
    corev1 "k8s.io/api/core/v1"
)
```

2. Initialize kubetracer in Your Controller:

```golang
type MyController struct {
    Client *kubetracer.TracingClient
    Logger logr.Logger
}

func (r *MyController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
    // Get the resource being reconciled
    pod := &corev1.Pod{}
    if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
        return reconcile.Result{}, client.IgnoreNotFound(err)
    }

    // Perform reconcile logic here
    // ...

    // Example: Trigger another resource update within the same trace context
    childPod := &corev1.Pod{}
    // Update or create the child resource
    if err := r.Client.Update(ctx, childPod); err != nil {
        return reconcile.Result{}, err
    }

    return reconcile.Result{}, nil
}
```

3. Create the Add Function

```golang
func Add(mgr manager.Manager, logger logr.Logger) error {
    // Setup the controller
    c, err := controller.New("my-controller", mgr, controller.Options{
        Reconciler: &MyController{
            Client: kubetracer.NewTracingClient(mgr.GetClient(), otel.Tracer("kubetracer"), logger),
            Logger: logger,
        },
    })
    if err != nil {
        return err
    }

    // Watch for changes to primary resource
    err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, predicates.IgnoreTraceAnnotationUpdatePredicate{})
    if err != nil {
        return err
    }

    return nil
}
```

4. Set Up the Manager in the Main Package:

```golang
package main

import (
    "os"
    "github.com/go-logr/logr"
    "github.com/kubetracer/kubetracer/controllers"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
    "sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
    // Set up the logger
    logger := zap.New(zap.UseDevMode(true))

    // Create a new manager
    mgr, err := manager.New(manager.GetConfigOrDie(), manager.Options{})
    if err != nil {
        logger.Error(err, "Unable to set up overall controller manager")
        os.Exit(1)
    }

    // Add the controller to the manager
    if err := controllers.Add(mgr, logger); err != nil {
        logger.Error(err, "Unable to add controller to manager")
        os.Exit(1)
    }

    // Start the manager
    if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
        logger.Error(err, "Unable to start manager")
        os.Exit(1)
    }
} 
```

## Contributing

We welcome contributions from the community! To get started, please read our contributing guidelines.
Fork the Repository: Click the "Fork" button at the top right of this page.
Clone Your Fork:

```bash
git clone https://github.com/kubetracer/kubetracer-go.git
cd kubetracer-go
```

Create a Branch:

```bash
git checkout -b feature/your-feature
```

Make Your Changes: Implement your feature or bug fix.
Commit Your Changes:

```bash
git commit -m "Add feature: your feature"
```

Push to Your Fork:

```bash
git push origin feature/your-feature
```

Create a Pull Request: Open a pull request to merge your changes into the main repository.

## License

kubetracer is licensed under the Apache License, Version 2.0. See LICENSE for the full license text.
