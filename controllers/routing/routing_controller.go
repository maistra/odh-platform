package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/opendatahub-io/odh-platform/pkg/resource/routing"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewPlatformRoutingReconciler(cli client.Client, log logr.Logger, routingComponent spi.RoutingComponent, config PlatformRoutingConfiguration) *PlatformRoutingReconciler {
	return &PlatformRoutingReconciler{
		Client:         cli,
		log:            log,
		component:      routingComponent,
		config:         config,
		templateLoader: routing.NewStaticTemplateLoader(),
	}
}

type reconcileRoutingFunc func(ctx context.Context, target *unstructured.Unstructured) error

// PlatformRoutingReconciler holds the controller configuration.
type PlatformRoutingReconciler struct {
	client.Client
	log            logr.Logger
	component      spi.RoutingComponent
	templateLoader spi.RoutingTemplateLoader
	config         PlatformRoutingConfiguration
}

type PlatformRoutingConfiguration struct {
	IngressSelectorLabel,
	IngressSelectorValue,
	IngressService,
	GatewayNamespace string
}

// +kubebuilder:rbac:groups="route.openshift.io",resources=routes,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=destinationrule,verbs=*

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformRoutingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcilers := []reconcileRoutingFunc{r.reconcileResources}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.component.CustomResourceType.GroupVersionKind)

	if err := r.Client.Get(ctx, req.NamespacedName, sourceRes); err != nil {
		if k8serr.IsNotFound(err) {
			r.log.Info("skipping reconcile. resource does not exist anymore", "resource", sourceRes.GroupVersionKind().String())

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed getting resource: %w", err)
	}

	r.log.Info("triggered route reconcile", "namespace", req.Namespace, "name", req.Name)

	var errs []error
	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, sourceRes))
	}

	return ctrl.Result{}, errors.Join(errs...)
}

func (r *PlatformRoutingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		For(&metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: r.component.CustomResourceType.GroupVersion().String(),
				Kind:       r.component.CustomResourceType.Kind,
			},
		}, builder.OnlyMetadata).
		Owns(&istionetworkingv1beta1.DestinationRule{}).
		Owns(&istionetworkingv1beta1.VirtualService{}).
		Owns(&istionetworkingv1beta1.Gateway{}).
		Owns(&openshiftroutev1.Route{}).
		Complete(r)
}
