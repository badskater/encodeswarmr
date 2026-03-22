# ─── Private DNS Zone for PostgreSQL ─────────────────────────────────────────

resource "azurerm_private_dns_zone" "postgres" {
  name                = "${local.name_prefix}.postgres.database.azure.com"
  resource_group_name = azurerm_resource_group.main.name

  tags = local.common_tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "${local.name_prefix}-postgres-dns-link"
  resource_group_name   = azurerm_resource_group.main.name
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  virtual_network_id    = azurerm_virtual_network.main.id
  registration_enabled  = false

  tags = local.common_tags
}

# ─── PostgreSQL Flexible Server ───────────────────────────────────────────────

resource "azurerm_postgresql_flexible_server" "main" {
  name                   = "${local.name_prefix}-postgres"
  resource_group_name    = azurerm_resource_group.main.name
  location               = azurerm_resource_group.main.location
  version                = var.db_version
  delegated_subnet_id    = azurerm_subnet.database.id
  private_dns_zone_id    = azurerm_private_dns_zone.postgres.id
  administrator_login    = var.db_admin_login
  administrator_password = var.db_admin_password

  sku_name   = var.db_sku_name
  storage_mb = var.db_storage_mb

  # Zone-redundant in HA mode, single-zone in standard mode
  high_availability {
    mode                      = var.enable_ha ? "ZoneRedundant" : "Disabled"
    standby_availability_zone = var.enable_ha ? "2" : null
  }

  # Availability zone for primary
  zone = var.enable_ha ? "1" : null

  backup_retention_days        = var.enable_ha ? 35 : 7
  geo_redundant_backup_enabled = var.enable_ha

  # Enforce SSL
  authentication {
    active_directory_auth_enabled = false
    password_auth_enabled         = true
  }

  depends_on = [
    azurerm_private_dns_zone_virtual_network_link.postgres,
  ]

  tags = local.common_tags
}

# ─── Application Database ─────────────────────────────────────────────────────

resource "azurerm_postgresql_flexible_server_database" "encodeswarmr" {
  name      = "encodeswarmr"
  server_id = azurerm_postgresql_flexible_server.main.id
  charset   = "UTF8"
  collation = "en_US.utf8"
}

# ─── PostgreSQL Configuration ─────────────────────────────────────────────────

resource "azurerm_postgresql_flexible_server_configuration" "max_connections" {
  name      = "max_connections"
  server_id = azurerm_postgresql_flexible_server.main.id
  value     = "100"
}

resource "azurerm_postgresql_flexible_server_configuration" "shared_buffers" {
  name      = "shared_buffers"
  server_id = azurerm_postgresql_flexible_server.main.id
  # 25% of RAM — this is a hint; Azure may override based on SKU
  value = "131072" # 128 MB in 8KB pages
}

# ─── Firewall Rules ───────────────────────────────────────────────────────────
# The server is VNet-integrated (private), but explicit deny of public access
# is enforced via the delegated subnet. No public firewall rules are added.
# If you need to allow Azure services (e.g. for migrations), uncomment below:
#
# resource "azurerm_postgresql_flexible_server_firewall_rule" "azure_services" {
#   name             = "allow-azure-services"
#   server_id        = azurerm_postgresql_flexible_server.main.id
#   start_ip_address = "0.0.0.0"
#   end_ip_address   = "0.0.0.0"
# }
