module "authz_platform" {
  source = "../../modules/authz-platform"

  chart_path       = abspath("../../../../deploy/helm/go-authorization-platform")
  image_tag        = var.image_tag
  openfga_url      = var.openfga_url
  openfga_store_id = var.openfga_store_id
  openfga_model_id = var.openfga_model_id
  database_url     = var.database_url
  labels           = { environment = "dev", owner = "platform" }
}
