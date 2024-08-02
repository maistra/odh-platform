package authorization

import (
	"context"
	"fmt"
	"reflect"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecurityv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PlatformAuthorizationReconciler) reconcilePeerAuthentication(ctx context.Context, target *unstructured.Unstructured) error {
	desired := createPeerAuthentication(r.authComponent.WorkloadSelector, target)
	found := &istiosecurityv1beta1.PeerAuthentication{}
	justCreated := false

	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if k8serr.IsNotFound(err) {
			errCreate := r.Create(ctx, desired)
			if client.IgnoreAlreadyExists(errCreate) != nil {
				return fmt.Errorf("unable to create PeerAuthentication: %w", errCreate)
			}

			justCreated = true
		} else {
			return fmt.Errorf("unable to fetch PeerAuthentication: %w", err)
		}
	}

	// Reconcile the Istio PeerAuthentication if it has been manually modified
	if !justCreated && !ComparePeerAuthentication(desired, found) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return fmt.Errorf("failed getting PeerAuthentication %s in namespace %s: %w", desired.Name, desired.Namespace, err)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels

			if errUpdate := r.Update(ctx, found); errUpdate != nil {
				return fmt.Errorf("failed updating PeerAuthentication: %w", errUpdate)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("unable to reconcile the PeerAuthentication: %w", err)
		}
	}

	return nil
}

func createPeerAuthentication(workloadSelector map[string]string, target *unstructured.Unstructured) *istiosecurityv1beta1.PeerAuthentication {
	policy := &istiosecurityv1beta1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:        target.GetName(),
			Namespace:   target.GetNamespace(),
			Labels:      metadata.ApplyStandard(target.GetLabels()),
			Annotations: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				targetToOwnerRef(target),
			},
		},
		Spec: v1beta1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{
				MatchLabels: workloadSelector,
			},
			Mtls: &v1beta1.PeerAuthentication_MutualTLS{Mode: v1beta1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}

	return policy
}

func ComparePeerAuthentication(peerAuth1, peerAuth2 *istiosecurityv1beta1.PeerAuthentication) bool {
	// .Spec contains MessageState from protobuf which has pragma.DoNotCopy (empty mutex slice)
	// go vet complains about copying mutex when calling DeepEquals on passed variables, when it tries to access it.
	// DeepCopy-ing solves this problem as it's using proto.Clone underneath. This implementation recreates mutex instead of
	// directly copying.
	// Alternatively we could break DeepEquals calls to individual .Spec exported fields, but that might mean ensuring we
	// always compare all relevant fields when API changes.
	peerSpec1 := peerAuth1.Spec.DeepCopy()
	peerSpec2 := peerAuth2.Spec.DeepCopy()

	return reflect.DeepEqual(peerAuth1.ObjectMeta.Labels, peerAuth2.ObjectMeta.Labels) &&
		reflect.DeepEqual(peerSpec1, peerSpec2)
}
