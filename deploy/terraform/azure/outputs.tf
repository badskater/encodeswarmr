# ─── Controller ───────────────────────────────────────────────────────────────

output "controller_public_ips" {
  description = "Public IP address(es) of the controller VM(s). In standard mode this is a single IP; in HA mode these are per-VM IPs (use the LB IP to reach the service)."
  value       = azurerm_public_ip.controller[*].ip_address
}

output "controller_web_url" {
  description = "URL for the controller web UI. Points to the load balancer in HA mode or the single controller in standard mode."
  value = var.enable_ha ? (
    "http://${azurerm_public_ip.lb[0].ip_address}:8080"
    ) : (
    "http://${azurerm_public_ip.controller[0].ip_address}:8080"
  )
}

output "controller_grpc_address" {
  description = "gRPC endpoint (host:port) for agent connections."
  value = var.enable_ha ? (
    "${azurerm_public_ip.lb[0].fqdn}:9443"
    ) : (
    "${azurerm_public_ip.controller[0].ip_address}:9443"
  )
}

# ─── Load Balancer (HA only) ──────────────────────────────────────────────────

output "load_balancer_ip" {
  description = "Public IP address of the Azure Load Balancer. Empty in standard (non-HA) mode."
  value       = var.enable_ha ? azurerm_public_ip.lb[0].ip_address : ""
}

output "load_balancer_fqdn" {
  description = "FQDN of the Azure Load Balancer. Empty in standard (non-HA) mode."
  value       = var.enable_ha ? azurerm_public_ip.lb[0].fqdn : ""
}

# ─── Database ─────────────────────────────────────────────────────────────────

output "postgresql_fqdn" {
  description = "Fully-qualified domain name of the PostgreSQL Flexible Server."
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "postgresql_database_name" {
  description = "Name of the application database."
  value       = azurerm_postgresql_flexible_server_database.distencoder.name
}

output "postgresql_connection_string" {
  description = "PostgreSQL connection string (password redacted — retrieve db-password secret from Key Vault)."
  value       = "postgres://${var.db_admin_login}:<db-password>@${azurerm_postgresql_flexible_server.main.fqdn}:5432/${azurerm_postgresql_flexible_server_database.distencoder.name}?sslmode=require"
  sensitive   = false
}

# ─── Storage ──────────────────────────────────────────────────────────────────

output "storage_account_name" {
  description = "Name of the Azure Storage Account used as shared NAS."
  value       = azurerm_storage_account.nas.name
}

output "storage_file_shares" {
  description = "Names of the Azure File Shares created for NAS mounts."
  value = {
    media   = azurerm_storage_share.media.name
    encodes = azurerm_storage_share.encodes.name
    temp    = azurerm_storage_share.temp.name
  }
}

output "storage_protocol" {
  description = "Protocol used for file shares (NFS for Premium tier, SMB for Standard)."
  value       = var.storage_account_tier == "Premium" ? "NFS" : "SMB"
}

# ─── Key Vault ────────────────────────────────────────────────────────────────

output "key_vault_name" {
  description = "Name of the Azure Key Vault storing mTLS certificates and secrets."
  value       = azurerm_key_vault.main.name
}

output "key_vault_uri" {
  description = "URI of the Azure Key Vault."
  value       = azurerm_key_vault.main.vault_uri
}

# ─── Agent VMSS ───────────────────────────────────────────────────────────────

output "agent_vmss_name" {
  description = "Name of the agent Virtual Machine Scale Set."
  value       = azurerm_linux_virtual_machine_scale_set.agents.name
}

# ─── Networking ───────────────────────────────────────────────────────────────

output "vnet_name" {
  description = "Name of the virtual network."
  value       = azurerm_virtual_network.main.name
}

output "resource_group_name" {
  description = "Name of the resource group containing all resources."
  value       = azurerm_resource_group.main.name
}

# ─── Deployment Summary ───────────────────────────────────────────────────────

output "deployment_summary" {
  description = "Quick-reference summary of the deployment."
  value = {
    environment        = var.environment
    ha_enabled         = var.enable_ha
    controller_count   = local.controller_count
    agent_count        = var.agent_count
    region             = var.location
    resource_group     = azurerm_resource_group.main.name
    web_ui             = var.enable_ha ? "http://${azurerm_public_ip.lb[0].ip_address}:8080" : "http://${azurerm_public_ip.controller[0].ip_address}:8080"
    grpc_endpoint      = var.enable_ha ? "${azurerm_public_ip.lb[0].fqdn}:9443" : "${azurerm_public_ip.controller[0].ip_address}:9443"
    postgres_fqdn      = azurerm_postgresql_flexible_server.main.fqdn
    storage_account    = azurerm_storage_account.nas.name
    key_vault          = azurerm_key_vault.main.name
  }
}
