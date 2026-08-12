terraform {
  required_version = ">= 1.15.0, < 1.16.0"
  backend "local" { path = ".terraform/state/dev.tfstate" }
  required_providers {
    kubernetes = { source = "hashicorp/kubernetes", version = "~> 3.2" }
    helm       = { source = "hashicorp/helm", version = "~> 3.2" }
  }
}

provider "kubernetes" { config_path = var.kubeconfig_path }
provider "helm" { kubernetes = { config_path = var.kubeconfig_path } }
