terraform {
  required_version = ">= 1.14.0, < 1.16.0"
  required_providers {
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 3.2" }
    helm       = { source = "hashicorp/helm", version = "~> 3.2" }
  }
}
