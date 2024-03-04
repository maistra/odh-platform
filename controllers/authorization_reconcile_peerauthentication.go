package controllers

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"istio.io/api/security/v1beta1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecv1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcilePeerAuthentication(ctx context.Context, service *v1.Service) error {

	desired := createPeerAuthentication(service)
	found := &istiosecv1beta1.PeerAuthentication{}
	justCreated := false

	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if apierrs.IsNotFound(err) {

			err = r.Create(ctx, desired)
			if err != nil && !apierrs.IsAlreadyExists(err) {

				return errors.Wrap(err, "unable to create PeerAuthentication")
			}

			justCreated = true
		} else {

			return errors.Wrap(err, "unable to fetch PeerAuthentication")
		}
	}

	// Reconcile the Istio PeerAuthentication if it has been manually modified
	if !justCreated && !ComparePeerAuthentication(desired, found) {

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return errors.Wrapf(err, "failed getting PeerAuthentication %s in namespace %s", desired.Name, desired.Namespace)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels
			// TODO: Merge Annotations?

			return errors.Wrap(r.Update(ctx, found), "failed updating PeerAuthentication")
		}); err != nil {

			return errors.Wrap(err, "unable to reconcile the PeerAuthentication")
		}
	}

	return nil
}

func createPeerAuthentication(service *v1.Service) *istiosecv1beta1.PeerAuthentication {
	policy := &istiosecv1beta1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:        service.Name,
			Namespace:   service.Namespace,
			Labels:      service.Labels,      // TODO: Where to fetch lables from
			Annotations: map[string]string{}, // TODO: where to fetch annotations from? part-of "service comp" or "platform?"
		},
		Spec: v1beta1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{
				MatchLabels: service.Spec.Selector,
			},
			Mtls: &v1beta1.PeerAuthentication_MutualTLS{Mode: v1beta1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
	return policy
}

func ComparePeerAuthentication(m1, m2 *istiosecv1beta1.PeerAuthentication) bool {
	return reflect.DeepEqual(m1.ObjectMeta.Labels, m2.ObjectMeta.Labels) &&
		reflect.DeepEqual(m1.Spec, m2.Spec)
}
