package routingctrl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	platformctrl "github.com/opendatahub-io/odh-platform/controllers"
	"github.com/opendatahub-io/odh-platform/pkg/routing"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"github.com/opendatahub-io/odh-platform/pkg/unstruct"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	name          = "routing"
	finalizerName = "routing.opendatahub.io/finalizer"
)

func New(cli client.Client, log logr.Logger, component spi.RoutingComponent, config spi.PlatformRoutingConfiguration) *Controller {
	return &Controller{
		active: true,
		Client: cli,
		log: log.WithValues(
			"controller", name,
			"component", component.ObjectReference.Kind,
		),
		component:      component,
		config:         config,
		templateLoader: routing.NewStaticTemplateLoader(),
	}
}

// Controller holds the routing controller configuration.
type Controller struct {
	client.Client
	active         bool
	log            logr.Logger
	component      spi.RoutingComponent
	templateLoader spi.RoutingTemplateLoader
	config         spi.PlatformRoutingConfiguration
}

// +kubebuilder:rbac:groups="route.openshift.io",resources=routes,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=destinationrules,verbs=*
// +kubebuilder:rbac:groups="",resources=services,verbs=*

// Reconcile ensures that the component has all required resources needed to use routing capability of the platform.
func (r *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.active {
		r.log.V(5).Info("controller is not active")

		return ctrl.Result{}, nil
	}

	reconcilers := []platformctrl.SubReconcileFunc{
		r.removeUnusedRoutingResources,
		r.createRoutingResources,
	}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.component.ObjectReference.GroupVersionKind)

	if err := r.Client.Get(ctx, req.NamespacedName, sourceRes); err != nil {
		if k8serr.IsNotFound(err) {
			r.log.Info("skipping reconcile. resource does not exist anymore", "resource", sourceRes.GroupVersionKind().String())

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed getting resource: %w", err)
	}

	r.log.Info("triggered routing reconcile", "namespace", req.Namespace, "name", req.Name)

	if unstruct.IsMarkedForDeletion(sourceRes) {
		return ctrl.Result{}, r.handleResourceDeletion(ctx, sourceRes)
	}

	var errs []error

	if errFinalizer := r.ensureResourceHasFinalizer(ctx, sourceRes); errFinalizer != nil {
		return ctrl.Result{}, fmt.Errorf("failed adding finalizer: %w", errFinalizer)
	}

	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, sourceRes))
	}

	errs = append(errs, unstruct.Patch(ctx, r.Client, sourceRes))

	return ctrl.Result{}, errors.Join(errs...)
}

func (r *Controller) Name() string {
	return name + "-" + strings.ToLower(r.component.ObjectReference.Kind)
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		// Ensures client is set - fall back to the one defined for the passed manager
		r.Client = mgr.GetClient()
	}

	// TODO(mvp) define predicates for labels, annotation and generation changes
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		Named(r.Name()).
		For(&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: r.component.ObjectReference.GroupVersion().String(),
				Kind:       r.component.ObjectReference.Kind,
			},
		}, builder.OnlyMetadata).
		Owns(&istionetworkingv1beta1.DestinationRule{}).
		Owns(&istionetworkingv1beta1.VirtualService{}).
		Owns(&istionetworkingv1beta1.Gateway{}).
		Owns(&openshiftroutev1.Route{}).
		Complete(r)
}

func (r *Controller) Activate() {
	r.active = true
}

func (r *Controller) Deactivate() {
	r.active = false
}
