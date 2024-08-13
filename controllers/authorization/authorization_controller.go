package authorization

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
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const ctrlName = "authorization"

func NewPlatformAuthorizationController(cli client.Client, log logr.Logger,
	component spi.AuthorizationComponent, config PlatformAuthorizationConfig) *PlatformAuthorizationController {
	return &PlatformAuthorizationController{
		active: true,
		Client: cli,
		log: log.WithValues(
			"controller", ctrlName,
			"component", component.ObjectReference.Kind,
		),
		config:         config,
		authComponent:  component,
		typeDetector:   authorization.NewAnnotationAuthTypeDetector(metadata.Annotations.AuthEnabled),
		hostExtractor:  authorization.NewExpressionHostExtractor(component.HostPaths),
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

// PlatformAuthorizationController holds the controller configuration.
type PlatformAuthorizationController struct {
	client.Client
	active         bool
	log            logr.Logger
	config         PlatformAuthorizationConfig
	authComponent  spi.AuthorizationComponent
	typeDetector   spi.AuthTypeDetector
	hostExtractor  spi.HostExtractor
	templateLoader spi.AuthConfigTemplateLoader
}

// +kubebuilder:rbac:groups=authorino.kuadrant.io,resources=authconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=authorizationpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=security.istio.io,resources=peerauthentications,verbs=get;list;watch;create;update;patch;delete

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformAuthorizationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.active {
		r.log.V(5).Info("controller is not active")

		return ctrl.Result{}, nil
	}

	reconcilers := []platformctrl.SubReconcileFunc{r.reconcileAuthConfig, r.reconcileAuthPolicy, r.reconcilePeerAuthentication}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.authComponent.ObjectReference.GroupVersionKind)

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

func (r *PlatformAuthorizationController) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		// Ensures client is set - fall back to the one defined for the passed manager
		r.Client = mgr.GetClient()
	}

	// TODO(mvp): define predicates so we do not reconcile unnecessarily
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		Named(ctrlName+"-"+strings.ToLower(r.authComponent.ObjectReference.Kind)).
		For(&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: r.authComponent.ObjectReference.GroupVersion().String(),
				Kind:       r.authComponent.ObjectReference.Kind,
			},
		}, builder.OnlyMetadata).
		Owns(&authorinov1beta2.AuthConfig{}).
		Owns(&istiosecurityv1beta1.AuthorizationPolicy{}).
		Owns(&istiosecurityv1beta1.PeerAuthentication{}).
		Complete(r)
}

func (r *PlatformAuthorizationController) Activate() {
	r.active = true
}

func (r *PlatformAuthorizationController) Deactivate() {
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
