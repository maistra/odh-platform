package authorization

import (
	"context"
	"fmt"
	"reflect"

	authorinov1beta2 "github.com/kuadrant/authorino/api/v1beta2"
	"github.com/opendatahub-io/odh-platform/pkg/env"
	"github.com/opendatahub-io/odh-platform/pkg/label"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PlatformAuthorizationReconciler) reconcileAuthConfig(ctx context.Context, target *unstructured.Unstructured) error {
	authType, err := r.typeDetector.Detect(ctx, target)
	if err != nil {
		return fmt.Errorf("could not detect authtype: %w", err)
	}

	templ, err := r.templateLoader.Load(ctx, authType, types.NamespacedName{Namespace: target.GetNamespace(), Name: target.GetName()})
	if err != nil {
		return fmt.Errorf("could not load template %s: %w", authType, err)
	}

	hosts := r.hostExtractor.Extract(target)

	desired, err := createAuthConfig(templ, hosts, target)
	if err != nil {
		return fmt.Errorf("could not create destired AuthConfig: %w", err)
	}

	found := &authorinov1beta2.AuthConfig{}
	justCreated := false

	err = r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, found)
	if err != nil {
		if k8serr.IsNotFound(err) {
			errCreate := r.Create(ctx, desired)
			if client.IgnoreAlreadyExists(errCreate) != nil {
				return fmt.Errorf("unable to create AuthConfig: %w", errCreate)
			}

			justCreated = true
		} else {
			return fmt.Errorf("unable to fetch AuthConfig: %w", err)
		}
	}

	// Reconcile the Authorino AuthConfig if it has been manually modified
	if !justCreated && !CompareAuthConfigs(desired, found) {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, found); err != nil {
				return fmt.Errorf("failed getting AuthConfig %s in namespace %s: %w", desired.Name, desired.Namespace, err)
			}

			found.Spec = *desired.Spec.DeepCopy()
			found.ObjectMeta.Labels = desired.ObjectMeta.Labels

			if errUpdate := r.Update(ctx, found); errUpdate != nil {
				return fmt.Errorf("failed updating AuthConfig: %w", errUpdate)
			}

			return nil
		}); err != nil {
			return fmt.Errorf("unable to reconcile the Authorino AuthConfig: %w", err)
		}
	}

	return nil
}

func createAuthConfig(templ authorinov1beta2.AuthConfig, hosts []string, target *unstructured.Unstructured) (*authorinov1beta2.AuthConfig, error) {
	authKey, authVal, err := env.GetAuthorinoLabel()
	if err != nil {
		return &authorinov1beta2.AuthConfig{}, fmt.Errorf("could not get authorino label selector: %w", err)
	}

	if templ.Annotations == nil {
		templ.Annotations = map[string]string{}
	}

	if templ.Labels == nil {
		templ.Labels = map[string]string{}
	}

	templ.Labels[authKey] = authVal

	stadLables := label.ApplyStandard(target.GetLabels())
	for k, v := range stadLables {
		if _, found := templ.Labels[k]; !found {
			templ.Labels[k] = v
		}
	}

	templ.Name = target.GetName()
	templ.Namespace = target.GetNamespace()
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
