# ---------------------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------------------

output "controller_private_ip" {
  description = "Private IP address of the controller VM (standard deployment only)."
  value       = var.enable_ha ? null : google_compute_instance.controller[0].network_interface[0].network_ip
}

output "controller_http_lb_ip" {
  description = "External IP of the HTTP load balancer fronting the controller (HA deployment only)."
  value       = var.enable_ha ? google_compute_forwarding_rule.http[0].ip_address : null
}

output "controller_grpc_lb_ip" {
  description = "External IP of the TCP/gRPC load balancer fronting the controller (HA deployment only)."
  value       = var.enable_ha ? google_compute_forwarding_rule.grpc[0].ip_address : null
}

output "controller_web_url" {
  description = "URL for the encodeswarmr web UI."
  value = var.enable_ha ? (
    "http://${google_compute_forwarding_rule.http[0].ip_address}:8080"
  ) : (
    "http://${google_compute_instance.controller[0].network_interface[0].network_ip}:8080 (private — use IAP tunnel)"
  )
}

output "cloud_sql_connection_name" {
  description = "Cloud SQL connection name (used with Cloud SQL Auth Proxy)."
  value       = google_sql_database_instance.main.connection_name
}

output "cloud_sql_private_ip" {
  description = "Private IP address of the Cloud SQL instance."
  value       = google_sql_database_instance.main.private_ip_address
}

output "filestore_ip" {
  description = "Filestore NFS server IP address."
  value       = google_filestore_instance.media.networks[0].ip_addresses[0]
}

output "filestore_share" {
  description = "Filestore NFS share path."
  value       = "/${google_filestore_instance.media.file_shares[0].name}"
}

output "agent_mig_name" {
  description = "Name of the agent Managed Instance Group."
  value       = google_compute_region_instance_group_manager.agents.name
}

output "controller_mig_name" {
  description = "Name of the controller Managed Instance Group (HA deployment only)."
  value       = var.enable_ha ? google_compute_region_instance_group_manager.controller[0].name : null
}

output "vpc_network_name" {
  description = "Name of the VPC network."
  value       = google_compute_network.main.name
}

output "controller_service_account" {
  description = "Email of the controller VM service account."
  value       = google_service_account.controller.email
}

output "agent_service_account" {
  description = "Email of the agent VM service account."
  value       = google_service_account.agent.email
}

output "ssh_iap_command_controller" {
  description = "Example gcloud command to SSH into a controller VM via IAP."
  value = var.enable_ha ? (
    "gcloud compute ssh --tunnel-through-iap --zone ${var.zone} <instance-name>"
  ) : (
    "gcloud compute ssh --tunnel-through-iap --zone ${var.zone} ${google_compute_instance.controller[0].name}"
  )
}
