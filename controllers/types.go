package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Activable interface {
	Activate()
	Deactivate()
}

type SetupWithManagerFunc func(mgr ctrl.Manager) error

type SubReconcileFunc func(ctx context.Context, target *unstructured.Unstructured) error
