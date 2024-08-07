package routing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	platformctrl "github.com/opendatahub-io/odh-platform/controllers"
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

const ctrlName = "routing"

func NewPlatformRoutingReconciler(cli client.Client, log logr.Logger, component spi.RoutingComponent, config spi.PlatformRoutingConfiguration) *PlatformRoutingReconciler {
	return &PlatformRoutingReconciler{
		Client: cli,
		log: log.WithValues(
			"controller", ctrlName,
			"component", component.CustomResourceType.Kind,
		),
		component:      component,
		config:         config,
		templateLoader: routing.NewStaticTemplateLoader(),
	}
}

// PlatformRoutingReconciler holds the controller configuration.
type PlatformRoutingReconciler struct {
	client.Client
	log            logr.Logger
	component      spi.RoutingComponent
	templateLoader spi.RoutingTemplateLoader
	config         spi.PlatformRoutingConfiguration
}

// +kubebuilder:rbac:groups="route.openshift.io",resources=routes,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=virtualservices,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=gateways,verbs=*
// +kubebuilder:rbac:groups="networking.istio.io",resources=destinationrule,verbs=*

// Reconcile ensures that the namespace has all required resources needed to be part of the Service Mesh of Open Data Hub.
func (r *PlatformRoutingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcilers := []platformctrl.SubReconcileFunc{r.reconcileResources}

	sourceRes := &unstructured.Unstructured{}
	sourceRes.SetGroupVersionKind(r.component.CustomResourceType.GroupVersionKind)

	if err := r.Client.Get(ctx, req.NamespacedName, sourceRes); err != nil {
		if k8serr.IsNotFound(err) {
			r.log.Info("skipping reconcile. resource does not exist anymore", "resource", sourceRes.GroupVersionKind().String())

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed getting resource: %w", err)
	}

	// TODO(mvp) if !sourceRes.GetDeletionTimestamp().IsZero() { handle removal of dependant resources }
	if !sourceRes.GetDeletionTimestamp().IsZero() {
		if err := r.HandleResourceDeletion(ctx, sourceRes); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to handle resource deletion: %w", err)
		}

		return ctrl.Result{}, nil
	}

	r.log.Info("triggered routing reconcile", "namespace", req.Namespace, "name", req.Name)

	var errs []error
	for _, reconciler := range reconcilers {
		errs = append(errs, reconciler(ctx, sourceRes))
	}

	if errUpdate := r.Update(ctx, sourceRes); errUpdate != nil {
		errs = append(errs, errUpdate)
	}

	return ctrl.Result{}, errors.Join(errs...)
}

func (r *PlatformRoutingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		// Ensures client is set - fall back to the one defined for the passed manager
		r.Client = mgr.GetClient()
	}

	// TODO(mvp) define predicates for labels, annotation and generation changes
	//nolint:wrapcheck //reason there is no point in wrapping it
	return ctrl.NewControllerManagedBy(mgr).
		Named(ctrlName+"-"+strings.ToLower(r.component.CustomResourceType.Kind)).
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
