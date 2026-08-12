variable "namespace" { type = string; default = "authz-system" }
variable "release_name" { type = string; default = "authz-platform" }
variable "chart_path" { type = string }
variable "image_repository" { type = string; default = "ghcr.io/mmmihaeel/go-authorization-platform" }
variable "image_tag" { type = string }
variable "openfga_url" { type = string }
variable "openfga_store_id" { type = string; sensitive = true }
variable "openfga_model_id" { type = string; sensitive = true }
variable "otlp_endpoint" { type = string; default = "otel-collector.observability.svc.cluster.local:4317" }
variable "labels" { type = map(string); default = {} }
