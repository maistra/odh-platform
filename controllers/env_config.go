package controllers

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

const (
	MeshNamespaceEnv       = "MESH_NAMESPACE"
	ControlPlaneEnv        = "CONTROL_PLANE_NAME"
	AuthorinoLabelSelector = "AUTHORINO_LABEL"
	AuthAudience           = "AUTH_AUDIENCE"
)

func getControlPlaneName() string {
	return getEnvOr(ControlPlaneEnv, "basic")
}

func getMeshNamespace() string {
	return getEnvOr(MeshNamespaceEnv, "istio-system")
}

func getAuthorinoLabel() ([]string, error) {
	label := getEnvOr(AuthorinoLabelSelector, "security.opendatahub.io/authorization-group=default")
	keyValue := strings.Split(label, "=")

	if len(keyValue) != 2 {
		return nil, errors.Errorf("Expected authorino label to be in key=value format, got [%s]", label)
	}

	return keyValue, nil
}

func getAuthAudience() []string {
	aud := getEnvOr(AuthAudience, "https://kubernetes.default.svc")
	audiences := strings.Split(aud, ",")

	for i := range audiences {
		audiences[i] = strings.TrimSpace(audiences[i])
	}

	return audiences
}

func getEnvOr(key, defaultValue string) string {
	if env, defined := os.LookupEnv(key); defined {
		return env
	}

	return defaultValue
}
