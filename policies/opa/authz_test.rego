package authz_test

import data.authz
import rego.v1

valid_input := {
    "subject": {
        "id": "alice",
        "tenant_id": "acme",
        "claims": {"account_suspended": false},
    },
    "resource": {
        "id": "roadmap",
        "tenant_id": "acme",
        "attributes": {"classification": "internal"},
    },
    "action": "view",
    "relation": "can_view",
    "relationship_allowed": true,
    "context": {
        "mfa": true,
        "device_trusted": true,
        "risk_score": 10,
        "purpose": "project_delivery",
    },
}

test_allows_valid_relationship_and_context if {
    result := authz.decision with input as valid_input
    result.allow
    count(result.reasons) == 0
}

test_denies_cross_tenant_access if {
    candidate := object.union(valid_input, {
        "resource": object.union(valid_input.resource, {"tenant_id": "globex"}),
    })
    result := authz.decision with input as candidate
    not result.allow
    "tenant_boundary_violation" in result.reasons
}

test_requires_mfa_for_delete if {
    candidate := object.union(valid_input, {
        "action": "delete",
        "context": object.union(valid_input.context, {"mfa": false}),
    })
    result := authz.decision with input as candidate
    not result.allow
    "mfa_required" in result.reasons
}

test_restricted_resource_requires_trusted_device_and_purpose if {
    candidate := object.union(valid_input, {
        "resource": object.union(valid_input.resource, {
            "attributes": {"classification": "restricted"},
        }),
        "context": object.union(valid_input.context, {
            "device_trusted": false,
            "purpose": "",
        }),
    })
    result := authz.decision with input as candidate
    not result.allow
    "trusted_device_required" in result.reasons
    "access_purpose_required" in result.reasons
}

test_relationship_result_cannot_be_overridden_by_context if {
    candidate := object.union(valid_input, {"relationship_allowed": false})
    result := authz.decision with input as candidate
    not result.allow
    "relationship_denied" in result.reasons
}
