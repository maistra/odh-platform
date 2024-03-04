package controllers

import (
	"context"

	"github.com/go-logr/logr"
	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/pkg/errors"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8serrs "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconcileAuthFunc func(ctx context.Context, service *v1.Service) error

// PlatformAuthorizationReconciler holds the controller configuration.
type PlatformAuthorizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=authorino.kuadrant.io,resources=authconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=peerauthentications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// TODO: Disable OAuthProxy injection in component Pods
// TODO: Disable OAuthProxy Route "hijack"
// TODO: Auto enable expose-route or remove NetworkPolicies.. or something else?

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformAuthorizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)

	reconcilers := []reconcileAuthFunc{r.reconcileAuthPolicy, r.reconcileAuthConfig, r.reconcilePeerAuthentication}

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

	// TODO: Update Service with authorization-group annotation that was used
	service := &v1.Service{}
	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Stopping reconciliation")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errors.Wrap(err, "failed getting service")
	}

	var errs []error
	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, service))
	}

	return ctrl.Result{}, k8serrs.NewAggregate(errs)
}

func (r *PlatformAuthorizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Service{}).
		Owns(&authorinov1beta2.AuthConfig{}).
		Owns(&istiosecv1beta1.AuthorizationPolicy{}).
		Owns(&istiosecv1beta1.PeerAuthentication{}).
		Complete(r)
}

func serviceToOwnerRef(service *v1.Service) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         service.APIVersion,
		Kind:               service.Kind,
		Name:               service.Name,
		UID:                service.UID,
		Controller:         false,
		BlockOwnerDeletion: *true,
	}
}
