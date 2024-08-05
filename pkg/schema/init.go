package schema

import (
	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	openshiftroutev1 "github.com/openshift/api/route/v1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// RegisterSchemes adds schemes of used resources to controller's scheme.
func RegisterSchemes(target *runtime.Scheme) {
	utilruntime.Must(metav1.AddMetaToScheme(target))
	utilruntime.Must(clientgoscheme.AddToScheme(target))
	utilruntime.Must(authorinov1beta2.AddToScheme(target))
	utilruntime.Must(istiosecurityv1beta1.AddToScheme(target))
	utilruntime.Must(istionetworkingv1beta1.AddToScheme(target))
	utilruntime.Must(openshiftroutev1.AddToScheme(target))
}
