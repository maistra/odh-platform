package controllers

import (
	"context"

	"github.com/go-logr/logr"
	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	openshiftv1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8serrs "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconcileRouteFunc func(ctx context.Context, route *openshiftv1.Route) error

// PlatformRouteReconciler holds the controller configuration.
type PlatformRouteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)

	reconcilers := []reconcileRouteFunc{r.reconcileAuthConfig}

	namespace := &v1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Namespace}, namespace); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Stopping reconciliation")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errors.Wrap(err, "failed getting namespace")
	}
	if _, isSet := namespace.Labels[AnnotationOpendatahub]; !isSet { // TODO: figure out correct discovery mode for namespace
		return ctrl.Result{}, nil
	}

	route := &openshiftv1.Route{}
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Stopping reconciliation")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errors.Wrap(err, "failed getting route")
	}

	var errs []error
	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, route))
	}

	return ctrl.Result{}, k8serrs.NewAggregate(errs)
}

func (r *PlatformRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		For(&openshiftv1.Route{}).
		Owns(&authorinov1beta2.AuthConfig{}).
		Complete(r)
}
