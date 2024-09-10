package spi

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HostExtractor attempts to extract Hosts from the given resource.
type HostExtractor func(res *unstructured.Unstructured) ([]string, error)

func NewAnnotationHostExtractor(separator string, annotationKeys ...string) HostExtractor {
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

func UnifiedHostExtractor(extractors ...HostExtractor) HostExtractor { //nolint:gocognit //reason Inlined functions to avoid package pollution, "function scoped"
	unique := func(in []string) []string {
		set := map[string]bool{}

		for _, elem := range in {
			set[elem] = true
		}

		unique := make([]string, len(set))

		i := 0

		for elem := range set {
			unique[i] = elem
			i++
		}

		return unique
	}

	isURL := func(host string) bool {
		return strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://")
	}

	appendHosts := func(hosts []string, foundHosts ...string) ([]string, error) {
		var errAllParse []error

		for _, foundHost := range foundHosts {
			if isURL(foundHost) {
				parsedURL, errParse := url.Parse(foundHost)
				if errParse != nil {
					errAllParse = append(errAllParse, fmt.Errorf("failed to parse URL %s: %w", foundHost, errParse))
				}

				hosts = append(hosts, parsedURL.Host)
			} else {
				hosts = append(hosts, foundHost)
			}
		}

		return hosts, errors.Join(errAllParse...)
	}

	return func(target *unstructured.Unstructured) ([]string, error) {
		var errAll []error

		combinedExtractedHosts := []string{}

		for _, extractor := range extractors {
			extractedHosts, err := extractor(target)
			if err != nil {
				errAll = append(errAll, err)

				continue
			}

			combinedExtractedHosts, err = appendHosts(combinedExtractedHosts, extractedHosts...)
			if err != nil {
				errAll = append(errAll, err)
			}
		}

		return unique(combinedExtractedHosts), errors.Join(errAll...)
	}
}

func NewPathExpressionExtractor(paths []string) HostExtractor {
	extractHosts := func(target *unstructured.Unstructured, splitPath []string) ([]string, error) { //nolint:unparam //reason Part of HostExtractor interface
		// extracting as string
		if foundHost, found, err := unstructured.NestedString(target.Object, splitPath...); err == nil && found {
			return []string{foundHost}, nil
		}

		// extracting as slice of strings
		if foundHosts, found, err := unstructured.NestedStringSlice(target.Object, splitPath...); err == nil && found {
			return foundHosts, nil
		}

		// TODO: Nothing found yet, move on no error?
		return []string{}, nil
	}

	return func(target *unstructured.Unstructured) ([]string, error) {
		var errExtract []error

		hosts := []string{}

		for _, path := range paths {
			splitPath := strings.Split(path, ".")
			extractedHosts, err := extractHosts(target, splitPath)

			if err != nil {
				errExtract = append(errExtract, fmt.Errorf("failed to extract hosts at path %s: %w", path, err))
			}

			hosts = append(hosts, extractedHosts...)
		}

		return hosts, errors.Join(errExtract...)
	}
}
