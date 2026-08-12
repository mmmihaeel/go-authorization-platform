resource "kubernetes_namespace_v1" "this" {
  metadata {
    name   = var.namespace
    labels = merge(var.labels, { "app.kubernetes.io/part-of" = "go-authorization-platform" })
  }
}

resource "kubernetes_secret_v1" "runtime" {
  metadata {
    name      = "authz-runtime"
    namespace = kubernetes_namespace_v1.this.metadata[0].name
  }
  data = {
    database-url = var.database_url
  }
  type = "Opaque"
}

resource "helm_release" "this" {
  name      = var.release_name
  namespace = kubernetes_namespace_v1.this.metadata[0].name
  chart     = var.chart_path
  atomic    = true
  wait      = true
  timeout   = 600

  values = [yamlencode({
    image = { repository = var.image_repository, tag = var.image_tag }
    config = {
      openfgaURL     = var.openfga_url
      openfgaStoreID = var.openfga_store_id
      openfgaModelID = var.openfga_model_id
      otlpEndpoint   = var.otlp_endpoint
    }
    runtimeSecret = { name = kubernetes_secret_v1.runtime.metadata[0].name }
    podLabels     = var.labels
  })]

  depends_on = [kubernetes_secret_v1.runtime]
}
