package labels

import (
	"github.com/opendatahub-io/odh-platform/pkg/metadata"
	"github.com/opendatahub-io/odh-platform/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AsOwner makes source object an owner of the target resource using labels.
// Those labels can be used to find all related resources across the cluster which
// are owned by the source object using Label selectors which simplifies query to kube-apiserver.
// This is particularly useful for garbage collection when source object is namespace-scoped
// and related resources are created in a different namespace or are cluster-scoped.
func AsOwner(source client.Object) []metadata.Option {
	ownerName := source.GetName()
	ownerKind := source.GetObjectKind().GroupVersionKind().Kind

	return []metadata.Option{
		OwnerName(ownerName),
		OwnerKind(ownerKind),
		OwnerUID(source.GetUID()),
	}
}

// StandardLabelsFrom constructs standard labels from the source object and returns them as metadata options.
func StandardLabelsFrom(source metav1.Object) []metadata.Option {
	stdLabels := standardLabelsFrom(source)

	var stdLabelsOpts []metadata.Option
	for i := range stdLabels {
		stdLabelsOpts = append(stdLabelsOpts, stdLabels[i])
	}

	return stdLabelsOpts
}

// AppendStandardLabelsFrom appends standard labels found in source object but only
// when they are not already present in the target object.
func AppendStandardLabelsFrom(source metav1.Object) *LabelAppender {
	return &LabelAppender{labels: standardLabelsFrom(source)}
}

// MatchingLabels returns a client.MatchingLabels selector for the provided labels.
func MatchingLabels(labels ...Label) client.MatchingLabels {
	matchingLabels := make(map[string]string)

	for _, l := range labels {
		matchingLabels[l.Key()] = l.Value()
	}

	return matchingLabels
}

// LabelAppender appends provided labels to the target object but only when they are not already present.
type LabelAppender struct {
	labels []Label
}

func (a *LabelAppender) ApplyToMeta(obj metav1.Object) {
	targetLabels := obj.GetLabels()

	for _, l := range a.labels {
		if _, exists := targetLabels[l.Key()]; !exists {
			l.ApplyToMeta(obj)
		}
	}
}

func standardLabelsFrom(source metav1.Object) []Label {
	sourceLabels := source.GetLabels()

	return []Label{
		AppPartOf(sourceLabels[AppName("").Key()]),
		AppComponent(sourceLabels[AppComponent("").Key()]),
		AppVersion(version.Version),
		AppManagedBy("odh-platform"),
	}
}
