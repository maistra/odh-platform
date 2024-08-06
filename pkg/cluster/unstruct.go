package cluster

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Apply(ctx context.Context, cli client.Client, objects []*unstructured.Unstructured, metaOptions ...metadata.Options) error {
	for _, source := range objects {
		for _, opt := range metaOptions {
			if err := opt(source); err != nil {
				return err
			}
		}

		target := source.DeepCopy()

		name := source.GetName()
		namespace := source.GetNamespace()

		errGet := cli.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: namespace}, target)
		if client.IgnoreNotFound(errGet) != nil {
			return fmt.Errorf("failed to get resource %s/%s: %w", namespace, name, errGet)
		}

		if k8serr.IsNotFound(errGet) {
			if errCreate := cli.Create(ctx, target); client.IgnoreAlreadyExists(errCreate) != nil { //nolint:gocritic //reason: we don't want to treat AlreadyExists as error here
				return fmt.Errorf("failed to create source %s/%s: %w", namespace, name, errCreate)
			}
		} else {
			if errUpdate := patchUsingApplyStrategy(ctx, cli, source, target); errUpdate != nil {
				return fmt.Errorf("failed to reconcile resource %s/%s: %w", namespace, name, errUpdate)
			}
		}
	}

	return nil
}

// patchUsingApplyStrategy performs server-side apply [1] patch to a Kubernetes resource.
// It treats the provided source as the desired state of the resource and attempts to
// reconcile the target resource to match this state. The function takes ownership of the
// fields specified in the target and will ensure they match desired state.
//
// [1] https://kubernetes.io/docs/reference/using-api/server-side-apply/
func patchUsingApplyStrategy(ctx context.Context, cli client.Client, source, target *unstructured.Unstructured) error {
	data, errJSON := source.MarshalJSON()
	if errJSON != nil {
		return fmt.Errorf("error converting yaml to json: %w", errJSON)
	}

	if errPatch := cli.Patch(ctx, target, client.RawPatch(k8stypes.ApplyPatchType, data), client.ForceOwnership, client.FieldOwner("odh-platform")); errPatch != nil {
		return fmt.Errorf("failed to apply patch to %s: %w", source.GroupVersionKind().String(), errPatch)
	}

	return nil
}
