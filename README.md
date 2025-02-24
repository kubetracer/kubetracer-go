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
