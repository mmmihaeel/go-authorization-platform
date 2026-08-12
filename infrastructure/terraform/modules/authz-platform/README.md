# `authz-platform` Terraform module

Deploys the Helm chart into a dedicated namespace and supplies environment-specific OpenFGA identifiers. It intentionally does **not** create a managed Kubernetes cluster, PostgreSQL database, or identity provider: those belong to separate platform modules and would make this portfolio repository cloud-specific.

Sensitive values should come from the CI secret store or an external secret controller. The example `kubernetes_secret_v1` is a local module contract, not a recommendation to commit secrets to Terraform state.
