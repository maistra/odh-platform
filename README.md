# ODH Platform

## Usage

### General flow diagram:

```mermaid
graph TD
A[ODH Operator] -->|Creates| B[ConfigMap]
B -->|Defines| C[Protected Resource]
D[ODH Platform] -->|Consumes| B
D -->|Watches| C
C -->|Upon creation| D
D -->|Creates| E[Authorino AuthConfigs]
D -->|Creates| F[Istio AuthorizationPolicies]
D -->|Creates| G[Istio PeerAuthentications]
H[ODH Component] -->|Creates instance of| C
```

The platform controller is deployed on the cluster automatically whenever a DSC component that indicates that it requires authorization is enabled.

### From the component developer perspective:
```mermaid
graph TD
A[Component Developer] -->|Defines| B[ProtectedResource in ODH Operator]
subgraph ProtectedResource
B1[Schema]
B2[WorkloadSelector]
B3[HostPaths]
B4[Ports]
B --> B1
B --> B2
B --> B3
B --> B4
end
A -->|Creates instance of| C[ProtectedResource in Cluster]
C -->|Watched by| D[ODH Platform]
D -->|Creates| E[Authorization Resources]
subgraph Authorization Resources
E1[Authorino AuthConfigs]
E2[Istio AuthorizationPolicies]
E3[Istio PeerAuthentications]
E --> E1
E --> E2
E --> E3
end
```

The developer needs to define the ProtectedResource in the ODH operator in order for the ODH platform controller to watch for the resources intended to be protected.
The ProtectedResource type looks like:
```go
type ProtectedResource struct {
Schema ResourceSchema json:"schema,omitempty"
WorkloadSelector map[string]string json:"workloadSelector,omitempty"
HostPaths []string json:"hostPaths,omitempty"
Ports []string json:"ports,omitempty"
}
```

Where Schema is a custom type:
```go
type ResourceSchema struct {
// GroupVersionKind specifies the group, version, and kind of the resource.
schema.GroupVersionKind `json:"gvk,omitempty"`
// Resources is the type of resource being protected, e.g., "pods", "services".
Resources string `json:"resources,omitempty"`
}
```