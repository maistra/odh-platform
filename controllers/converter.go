package controllers

import (
	"bytes"

	"github.com/manifestival/manifestival"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"
)

func ConvertToStructuredResource(yamlContent []byte, out interface{}, opts ...manifestival.Option) error {
	reader := bytes.NewReader(yamlContent)

	manifest, err := manifestival.ManifestFrom(manifestival.Reader(reader), opts...)
	if err != nil {
		return errors.Wrap(err, "failed reading manifest")
	}

	s := scheme.Scheme
	RegisterSchemes(s)

	if err := s.Convert(&manifest.Resources()[0], out, nil); err != nil {
		return errors.Wrap(err, "failed converting manifest")
	}

	return nil
}
