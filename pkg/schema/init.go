package schema

import (
	authorino "github.com/kuadrant/authorino/api/v1beta2"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// RegisterSchemes adds schemes of used resources to controller's scheme.
func RegisterSchemes(s *runtime.Scheme) {
	utilruntime.Must(metav1.AddMetaToScheme(s))
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(authorino.AddToScheme(s))
	utilruntime.Must(istiosecurityv1beta1.AddToScheme(s))
}
