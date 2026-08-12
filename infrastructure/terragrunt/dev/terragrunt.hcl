terraform {
  source = "../../terraform/environments/dev"
}

inputs = {
  image_tag = get_env("AUTHZ_IMAGE_TAG", "0.1.0")
  openfga_store_id = get_env("OPENFGA_STORE_ID")
  openfga_model_id = get_env("OPENFGA_MODEL_ID")
  database_url = get_env("DATABASE_URL")
}
