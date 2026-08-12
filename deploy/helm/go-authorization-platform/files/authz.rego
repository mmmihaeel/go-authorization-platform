package authz

import rego.v1

default allow := false

subject_claims := object.get(input.subject, "claims", {})
resource_attributes := object.get(input.resource, "attributes", {})
request_context := object.get(input, "context", {})

deny contains "relationship_denied" if {
    not input.relationship_allowed
}

deny contains "tenant_boundary_violation" if {
    input.subject.tenant_id != input.resource.tenant_id
}

deny contains "subject_suspended" if {
    object.get(subject_claims, "account_suspended", false)
}

deny contains "risk_score_too_high" if {
    object.get(request_context, "risk_score", 0) > 70
}

deny contains "mfa_required" if {
    input.action in {"delete", "manage"}
    not object.get(request_context, "mfa", false)
}

deny contains "trusted_device_required" if {
    object.get(resource_attributes, "classification", "internal") == "restricted"
    not object.get(request_context, "device_trusted", false)
}

deny contains "access_purpose_required" if {
    object.get(resource_attributes, "classification", "internal") == "restricted"
    object.get(request_context, "purpose", "") == ""
}

allow if {
    count(deny) == 0
}

decision := {
    "allow": allow,
    "reasons": sort(deny),
}
