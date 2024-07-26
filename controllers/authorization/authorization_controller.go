package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/controllers"
	"github.com/opendatahub-io/odh-platform/pkg/env"
	"github.com/opendatahub-io/odh-platform/pkg/resource"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewPlatformAuthorizationReconciler(cli client.Client, log logr.Logger, authComponent spi.AuthorizationComponent) *PlatformAuthorizationReconciler {
	return &PlatformAuthorizationReconciler{
		Client:         cli,
		log:            log,
		authComponent:  authComponent,
		typeDetector:   resource.NewAnnotationAuthTypeDetector(controllers.AnnotationAuthEnabled),
		hostExtractor:  resource.NewExpressionHostExtractor(authComponent.HostPaths),
		templateLoader: resource.NewConfigMapTemplateLoader(cli, resource.NewStaticTemplateLoader(env.GetAuthAudience())),
	}
}

type reconcileAuthFunc func(ctx context.Context, target *unstructured.Unstructured) error

// PlatformAuthorizationReconciler holds the controller configuration.
type PlatformAuthorizationReconciler struct {
	client.Client
	log            logr.Logger
	authComponent  spi.AuthorizationComponent
	typeDetector   spi.AuthTypeDetector
	hostExtractor  spi.HostExtractor
	templateLoader spi.AuthConfigTemplateLoader
}

// +kubebuilder:rbac:groups=authorino.kuadrant.io,resources=authconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=peerauthentications,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformAuthorizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcilers := []reconcileAuthFunc{r.reconcileAuthConfig, r.reconcileAuthPolicy, r.reconcilePeerAuthentication}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.authComponent.CustomResourceType.GroupVersionKind)

	if err := r.Client.Get(ctx, req.NamespacedName, sourceRes); err != nil {
		if k8serr.IsNotFound(err) {
			r.log.Info("skipping reconcile. resource does not exist anymore", "resource", sourceRes.GroupVersionKind().String())

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed getting resource: %w", err)
	}

	r.log.Info("triggered auth reconcile", "namespace", req.Namespace, "name", req.Name)

	var errs []error
	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, sourceRes))
	}

	return ctrl.Result{}, errors.Join(errs...)
}

func (r *PlatformAuthorizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		For(&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: r.authComponent.CustomResourceType.GroupVersion().String(),
				Kind:       r.authComponent.CustomResourceType.Kind,
			},
		}, builder.OnlyMetadata).
		Owns(&authorinov1beta2.AuthConfig{}).
		Owns(&istiosecurityv1beta1.AuthorizationPolicy{}).
		Owns(&istiosecurityv1beta1.PeerAuthentication{}).
		Complete(r)
}

func targetToOwnerRef(obj *unstructured.Unstructured) metav1.OwnerReference {
	controller := true

	return metav1.OwnerReference{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
		Controller: &controller,
	}
}
