variable "kubeconfig_path" { type = string; default = "~/.kube/config" }
variable "image_tag" { type = string; default = "0.1.0" }
variable "openfga_url" { type = string; default = "http://openfga.authz-system.svc.cluster.local:8080" }
variable "openfga_store_id" { type = string; sensitive = true }
variable "openfga_model_id" { type = string; sensitive = true }
