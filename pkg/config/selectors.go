package config

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResolveSelectors uses golang template engine to resolve the expressions in the `selectorExpressions` map using
// `source` as a data input. Both the keys and values are resolved against the source data.
//
// Note: expressions are resolved against the source using lowercase keys
//
// Example source:
//
//	  kind: Service
//	  metadata:
//		   name: MyService
//
// Example selectorExpressions:
//
//	 map[string]{string} {
//		  "routing.opendatahub.io/{{.kind}}": "{{.metadata.name}}", // > "routing.opendatahub.io/Service": "MyService"
//	 }
func ResolveSelectors(selectorExpressions map[string]string, source *unstructured.Unstructured) (map[string]string, error) {
	resolved := make(map[string]string, len(selectorExpressions))
	mainTemplate := template.New("unused_name").Option("missingkey=error")

	for key, val := range selectorExpressions {
		var err error

		resolvedKey := key
		if strings.Contains(key, "{{") {
			resolvedKey, err = resolve(mainTemplate, key, source)
			if err != nil {
				return nil, fmt.Errorf("could not resolve key %s: %w", key, err)
			}
		}

		resolvedVal := val
		if strings.Contains(val, "{{") {
			resolvedVal, err = resolve(mainTemplate, val, source)
			if err != nil {
				return nil, fmt.Errorf("could not resolve value %s: %w", val, err)
			}
		}

		resolved[resolvedKey] = resolvedVal
	}

	return resolved, nil
}

func resolve(templ *template.Template, textTemplate string, source *unstructured.Unstructured) (string, error) {
	tmpl, err := templ.Parse(textTemplate)
	if err != nil {
		return "", fmt.Errorf("could not parse template: %w", err)
	}

	var buff bytes.Buffer

	if err := tmpl.Execute(&buff, source.Object); err != nil {
		return "", fmt.Errorf("could not execute template: %w", err)
	}

	return buff.String(), nil
}
