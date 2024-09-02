package authzctrl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	platformctrl "github.com/opendatahub-io/odh-platform/controllers"
	"github.com/opendatahub-io/odh-platform/pkg/authorization"
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/pkg/metadata/annotations"
	"github.com/opendatahub-io/odh-platform/pkg/platform"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const name = "authorization"

func New(cli client.Client, log logr.Logger,
	protectedResource platform.ProtectedResource, config PlatformAuthorizationConfig) *Controller {
	return &Controller{
		active: true,
		Client: cli,
		log: log.WithValues(
			"controller", name,
			"component", protectedResource.ResourceReference.Kind,
		),
		config:            config,
		protectedResource: protectedResource,
		typeDetector:      authorization.NewAnnotationAuthTypeDetector(annotations.AuthEnabled("").Key()),
		// TODO: Evaluate passing in the hostExtractor to avoid coupling the authorizaiton/routing packages
		hostExtractor: spi.UnifiedHostExtractor(
			spi.NewPathExpressionExtractor(protectedResource.HostPaths),
			spi.NewAnnotationHostExtractor(";", metadata.Keys(annotations.RoutingAddressesExternal(""), annotations.RoutingAddressesPublic(""))...)),
		templateLoader: authorization.NewConfigMapTemplateLoader(cli, authorization.NewStaticTemplateLoader(config.Audiences)),
	}
}

type PlatformAuthorizationConfig struct {
	// Label in a format of key=value. It's used to target created AuthConfig by Authorino instance.
	Label string
	// Audiences is a list of audiences that will be used in the AuthConfig template when performing TokenReview.
	Audiences []string
	// ProviderName is the name of the registered external authorization provider in Service Mesh.
	ProviderName string
}

// Controller holds the authorization controller configuration.
type Controller struct {
	client.Client
	active            bool
	log               logr.Logger
	config            PlatformAuthorizationConfig
	protectedResource platform.ProtectedResource
	typeDetector      authorization.AuthTypeDetector
	hostExtractor     spi.HostExtractor
	templateLoader    authorization.AuthConfigTemplateLoader
}

// +kubebuilder:rbac:groups=authorino.kuadrant.io,resources=authconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures that the component has all required resources needed to use authorization capability of the platform.
func (r *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.active {
		r.log.V(5).Info("controller is not active")

		return ctrl.Result{}, nil
	}

	// NOTE: r.reconcilePeerAuthentication removed in https://github.com/maistra/odh-platform/pull/53
	// Revert if removal thesis breaks down
	reconcilers := []platformctrl.SubReconcileFunc{r.reconcileAuthConfig, r.reconcileAuthPolicy}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.protectedResource.ResourceReference.GroupVersionKind)

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

func (r *Controller) Name() string {
	return name + "-" + strings.ToLower(r.protectedResource.ResourceReference.Kind)
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		// Ensures client is set - fall back to the one defined for the passed manager
		r.Client = mgr.GetClient()
	}

	// TODO(mvp): define predicates so we do not reconcile unnecessarily
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		Named(r.Name()).
		For(&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: r.protectedResource.ResourceReference.GroupVersion().String(),
				Kind:       r.protectedResource.ResourceReference.Kind,
			},
		}, builder.OnlyMetadata).
		Owns(&authorinov1beta2.AuthConfig{}).
		Owns(&istiosecurityv1beta1.AuthorizationPolicy{}).
		Complete(r)
}

func (r *Controller) Activate() {
	r.active = true
}

func (r *Controller) Deactivate() {
	r.active = false
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
