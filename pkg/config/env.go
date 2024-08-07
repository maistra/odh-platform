package config

import (
	"os"
	"strings"
)

const (
	AuthAudience              = "AUTH_AUDIENCE"
	AuthProvider              = "AUTH_PROVIDER"
	RouteGatewayNamespace     = "ROUTE_GATEWAY_NAMESPACE"
	RouteGatewayService       = "ROUTE_GATEWAY_SERVICE"
	RouteIngressSelectorKey   = "ROUTE_INGRESS_SELECTOR_KEY"
	RouteIngressSelectorValue = "ROUTE_INGRESS_SELECTOR_VALUE"
	AuthorinoLabelSelector    = "AUTHORINO_LABEL"
	ConfigCapabilities        = "CONFIG_CAPABILITIES"
)

func GetAuthorinoLabel() string {
	return getEnvOr(AuthorinoLabelSelector, "security.opendatahub.io/authorization-group=default")
}

func GetAuthProvider() string {
	return getEnvOr(AuthProvider, "opendatahub-auth-provider")
}

func GetAuthAudience() []string {
	aud := getEnvOr(AuthAudience, "https://kubernetes.default.svc")
	audiences := strings.Split(aud, ",")

	for i := range audiences {
		audiences[i] = strings.TrimSpace(audiences[i])
	}

	return audiences
}

func GetConfigFile() string {
	return getEnvOr(ConfigCapabilities, "/tmp/platform-capabilities")
}

func GetGatewayNamespace() string {
	return getEnvOr(RouteGatewayNamespace, "opendatahub-services")
}

func GetGatewayService() string {
	return getEnvOr(RouteGatewayService, "opendatahub-ingress-router")
}

func GetIngressSelectorKey() string {
	return getEnvOr(RouteIngressSelectorKey, "istio")
}

func GetIngressSelectorValue() string {
	return getEnvOr(RouteIngressSelectorValue, "opendatahub-ingress-gateway")
}

func getEnvOr(key, defaultValue string) string {
	if env, defined := os.LookupEnv(key); defined {
		return env
	}

	return defaultValue
}
