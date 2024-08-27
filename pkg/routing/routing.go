package routing

import (
	"bytes"
	"context"
	_ "embed" // needed for go:embed directive
	"fmt"
	"strings"
	"text/template"

	"github.com/opendatahub-io/odh-platform/pkg/schema"
	"github.com/opendatahub-io/odh-platform/pkg/spi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

//go:embed template/routing_public.yaml
var publicRouteTemplate []byte

//go:embed template/routing_external.yaml
var externalRouteTemplate []byte

type staticTemplateLoader struct {
}

func NewStaticTemplateLoader() spi.RoutingTemplateLoader {
	return &staticTemplateLoader{}
}

func (s *staticTemplateLoader) Load(_ context.Context, routeType spi.RouteType, key types.NamespacedName, data spi.RoutingTemplateData) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	var templateContent []byte

	switch routeType {
	case spi.PublicRoute:
		templateContent = publicRouteTemplate
	case spi.ExternalRoute:
		templateContent = externalRouteTemplate
	default:
		templateContent = make([]byte, 0)
	}

	resolvedTemplates, err := s.resolveTemplate(templateContent, data)
	if err != nil {
		return nil, fmt.Errorf("could not resolve routing template: %w", err)
	}

	resolvedSplitTemplates := strings.Split(string(resolvedTemplates), "---")
	for _, resolvedTemplate := range resolvedSplitTemplates {
		resource := &unstructured.Unstructured{}

		if errConvert := schema.ConvertToStructuredResource([]byte(resolvedTemplate), resource); errConvert != nil {
			return nil, fmt.Errorf("could not load routing template: %w", err)
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (s *staticTemplateLoader) resolveTemplate(tmpl []byte, data spi.RoutingTemplateData) ([]byte, error) {
	engine, err := template.New("routing").Parse(string(tmpl))
	if err != nil {
		return []byte{}, fmt.Errorf("could not create template engine: %w", err)
	}

	buf := new(bytes.Buffer)

	err = engine.Execute(buf, data)
	if err != nil {
		return []byte{}, fmt.Errorf("could not execute template: %w", err)
	}

	return buf.Bytes(), nil
}

func NewAnnotationHostExtractor(separator string, annotationKeys ...string) spi.HostExtractor {
	return func(target *unstructured.Unstructured) ([]string, error) {
		hosts := []string{}

		for _, annKey := range annotationKeys {
			if val, found := target.GetAnnotations()[annKey]; found {
				hs := strings.Split(val, separator)
				hosts = append(hosts, hs...)
			}
		}

		return hosts, nil
	}
}
