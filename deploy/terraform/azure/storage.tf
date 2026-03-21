# ─── Storage Account ──────────────────────────────────────────────────────────
# NFS file shares require Premium tier + FileStorage kind.
# SMB shares work with Standard tier.

resource "random_string" "storage_suffix" {
  length  = 6
  upper   = false
  special = false
}

resource "azurerm_storage_account" "nas" {
  name                = "distenc${var.environment}${random_string.storage_suffix.result}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  # Premium FileStorage is required for NFS protocol support
  account_tier             = var.storage_account_tier
  account_replication_type = var.enable_ha ? "ZRS" : "LRS"
  account_kind             = var.storage_account_tier == "Premium" ? "FileStorage" : "StorageV2"

  # Disable public access — accessed via private endpoint
  public_network_access_enabled = false

  # Require secure transfer
  https_traffic_only_enabled = true
  min_tls_version            = "TLS1_2"

  # NFS requires the large file shares feature (Premium already supports it)
  large_file_share_enabled = var.storage_account_tier == "Standard" ? true : null

  network_rules {
    default_action = "Deny"
    bypass         = ["AzureServices"]
    virtual_network_subnet_ids = [
      azurerm_subnet.controller.id,
      azurerm_subnet.agent.id,
    ]
  }

  tags = local.common_tags
}

# ─── File Shares ──────────────────────────────────────────────────────────────

resource "azurerm_storage_share" "media" {
  name               = "media"
  storage_account_id = azurerm_storage_account.nas.id
  quota              = var.file_share_quota_gb

  # Use NFS for Premium, SMB for Standard
  enabled_protocol = var.storage_account_tier == "Premium" ? "NFS" : "SMB"
}

resource "azurerm_storage_share" "encodes" {
  name               = "encodes"
  storage_account_id = azurerm_storage_account.nas.id
  quota              = var.file_share_quota_gb

  enabled_protocol = var.storage_account_tier == "Premium" ? "NFS" : "SMB"
}

resource "azurerm_storage_share" "temp" {
  name               = "temp"
  storage_account_id = azurerm_storage_account.nas.id
  # Temp share can be smaller
  quota = 512

  enabled_protocol = var.storage_account_tier == "Premium" ? "NFS" : "SMB"
}

# ─── Private DNS Zone for Storage ─────────────────────────────────────────────

resource "azurerm_private_dns_zone" "storage_file" {
  name                = "privatelink.file.core.windows.net"
  resource_group_name = azurerm_resource_group.main.name

  tags = local.common_tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "storage_file" {
  name                  = "${local.name_prefix}-storage-dns-link"
  resource_group_name   = azurerm_resource_group.main.name
  private_dns_zone_name = azurerm_private_dns_zone.storage_file.name
  virtual_network_id    = azurerm_virtual_network.main.id
  registration_enabled  = false

  tags = local.common_tags
}

# ─── Private Endpoint for Storage ─────────────────────────────────────────────

resource "azurerm_private_endpoint" "storage" {
  name                = "${local.name_prefix}-storage-pe"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  subnet_id           = azurerm_subnet.storage.id

  private_service_connection {
    name                           = "${local.name_prefix}-storage-psc"
    private_connection_resource_id = azurerm_storage_account.nas.id
    is_manual_connection           = false
    subresource_names              = ["file"]
  }

  private_dns_zone_group {
    name                 = "storage-dns-zone-group"
    private_dns_zone_ids = [azurerm_private_dns_zone.storage_file.id]
  }

  tags = local.common_tags
}
