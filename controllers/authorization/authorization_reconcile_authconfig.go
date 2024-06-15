package authorization

import (
	"context"
	"reflect"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/env"
	"github.com/opendatahub-io/odh-platform/pkg/label"
	"github.com/pkg/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (r *PlatformAuthorizationReconciler) reconcileAuthConfig(ctx context.Context, target *unstructured.Unstructured) error {
	authType, err := r.typeDetector.Detect(ctx, target)
	if err != nil {
		return errors.Wrap(err, "could not detect authtype")
	}

	templ, err := r.templateLoader.Load(ctx, authType, types.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()})
	if err != nil {
		return errors.Wrap(err, "could not load template "+string(authType))
	}

	hosts := r.hostExtractor.Extract(target)

	desired, err := createAuthConfig(templ, hosts, target)
	if err != nil {
		return errors.Wrap(err, "could not create destired AuthConfig")
	}

	found := &authorinov1beta2.AuthConfig{}
	justCreated := false

	err = r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if apierrs.IsNotFound(err) {
			err = r.Create(ctx, desired)
			if err != nil && !apierrs.IsAlreadyExists(err) {
				return errors.Wrap(err, "unable to create AuthConfig")
			}

			justCreated = true
		} else {
			return errors.Wrap(err, "unable to fetch AuthConfig")
		}
	}

	// Reconcile the Authorino AuthConfig if it has been manually modified
	if !justCreated && !CompareAuthConfigs(desired, found) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return errors.Wrapf(err, "failed getting AuthConfig %s in namespace %s", desired.Name, desired.Namespace)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels

			return errors.Wrap(r.Update(ctx, found), "failed updating AuthConfig")
		}); err != nil {
			return errors.Wrap(err, "unable to reconcile the Authorino AuthConfig")
		}
	}

	return nil
}

func createAuthConfig(templ authorinov1beta2.AuthConfig, hosts []string, target *unstructured.Unstructured) (*authorinov1beta2.AuthConfig, error) {
	authKey, authVal, err := env.GetAuthorinoLabel()
	if err != nil {
		return &authorinov1beta2.AuthConfig{}, errors.Wrap(err, "could not get authorino label selcetor")
	}

	labels := target.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[authKey] = authVal
	templ.Name = target.GetName()
	templ.Namespace = target.GetNamespace()
	templ.Labels = label.ApplyStandard(target.GetLabels())
	templ.Annotations = map[string]string{}
	templ.Spec.Hosts = hosts
	templ.OwnerReferences = []metav1.OwnerReference{
		targetToOwnerRef(target),
	}

	return &templ, nil
}

func CompareAuthConfigs(m1, m2 *authorinov1beta2.AuthConfig) bool {
	return reflect.DeepEqual(m1.ObjectMeta.Labels, m2.ObjectMeta.Labels) &&
		reflect.DeepEqual(m1.Spec, m2.Spec)
}
