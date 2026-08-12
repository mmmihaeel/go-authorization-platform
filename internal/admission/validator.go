package admission

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProtectedAnnotation = "authz.platform.io/protected"
	PolicyRefAnnotation = "authz.platform.io/policy-ref"
	SidecarName         = "authorization-sidecar"
)

type workloadEnvelope struct {
	Metadata metadata `json:"metadata"`
	Spec     struct {
		Containers []container `json:"containers"`
		Template   struct {
			Metadata metadata `json:"metadata"`
			Spec     struct {
				Containers []container `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type metadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations"`
}

type container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type Result struct {
	Allowed bool
	Message string
}

func Validate(request *Request) Result {
	if request == nil {
		return Result{Allowed: false, Message: "admission request is missing"}
	}
	var workload workloadEnvelope
	if err := json.Unmarshal(request.Object, &workload); err != nil {
		return Result{Allowed: false, Message: fmt.Sprintf("decode workload: %v", err)}
	}

	annotations := workload.Metadata.Annotations
	containers := workload.Spec.Containers
	if strings.EqualFold(request.Kind.Kind, "Deployment") || strings.EqualFold(request.Kind.Kind, "StatefulSet") || strings.EqualFold(request.Kind.Kind, "DaemonSet") {
		annotations = workload.Spec.Template.Metadata.Annotations
		containers = workload.Spec.Template.Spec.Containers
	}

	if !strings.EqualFold(strings.TrimSpace(annotations[ProtectedAnnotation]), "true") {
		return Result{Allowed: true}
	}
	if strings.TrimSpace(annotations[PolicyRefAnnotation]) == "" {
		return Result{Allowed: false, Message: "protected workload must declare authz.platform.io/policy-ref"}
	}
	for _, item := range containers {
		if item.Name == SidecarName {
			return Result{Allowed: true}
		}
	}
	return Result{Allowed: false, Message: "protected workload must include the authorization-sidecar container"}
}
