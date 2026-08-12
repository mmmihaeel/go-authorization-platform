output "namespace" { value = kubernetes_namespace_v1.this.metadata[0].name }
output "release_name" { value = helm_release.this.name }
output "release_status" { value = helm_release.this.status }
