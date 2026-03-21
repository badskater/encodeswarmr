# ─── Key Vault ────────────────────────────────────────────────────────────────
# Stores mTLS certificates (CA, controller, agent) and application secrets
# (DB password, session secret).
# Controller and agent VMs authenticate via System-Assigned Managed Identity.

resource "random_string" "kv_suffix" {
  length  = 6
  upper   = false
  special = false
}

resource "azurerm_key_vault" "main" {
  name                = "distenc-${var.environment}-${random_string.kv_suffix.result}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  tenant_id           = data.azurerm_client_config.current.tenant_id

  sku_name = "standard"

  # Soft-delete and purge protection guard against accidental permanent deletion
  soft_delete_retention_days = 7
  purge_protection_enabled   = var.enable_ha

  # Disable public network access — all access via VNet / managed identities
  public_network_access_enabled = false

  network_acls {
    default_action             = "Deny"
    bypass                     = ["AzureServices"]
    virtual_network_subnet_ids = [
      azurerm_subnet.controller.id,
      azurerm_subnet.agent.id,
    ]
    ip_rules = []
  }

  tags = local.common_tags
}

# ─── Private DNS Zone for Key Vault ──────────────────────────────────────────

resource "azurerm_private_dns_zone" "keyvault" {
  name                = "privatelink.vaultcore.azure.net"
  resource_group_name = azurerm_resource_group.main.name

  tags = local.common_tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "keyvault" {
  name                  = "${local.name_prefix}-kv-dns-link"
  resource_group_name   = azurerm_resource_group.main.name
  private_dns_zone_name = azurerm_private_dns_zone.keyvault.name
  virtual_network_id    = azurerm_virtual_network.main.id
  registration_enabled  = false

  tags = local.common_tags
}

resource "azurerm_private_endpoint" "keyvault" {
  name                = "${local.name_prefix}-kv-pe"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  subnet_id           = azurerm_subnet.storage.id

  private_service_connection {
    name                           = "${local.name_prefix}-kv-psc"
    private_connection_resource_id = azurerm_key_vault.main.id
    is_manual_connection           = false
    subresource_names              = ["vault"]
  }

  private_dns_zone_group {
    name                 = "kv-dns-zone-group"
    private_dns_zone_ids = [azurerm_private_dns_zone.keyvault.id]
  }

  tags = local.common_tags
}

# ─── Terraform Operator Access Policy ────────────────────────────────────────
# Allows the identity running Terraform to seed initial secrets.

resource "azurerm_key_vault_access_policy" "terraform_operator" {
  key_vault_id = azurerm_key_vault.main.id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = data.azurerm_client_config.current.object_id

  secret_permissions = [
    "Get",
    "List",
    "Set",
    "Delete",
    "Purge",
    "Recover",
  ]

  certificate_permissions = [
    "Get",
    "List",
    "Import",
    "Create",
    "Delete",
    "Purge",
    "Recover",
  ]
}

# ─── Seed Secrets ─────────────────────────────────────────────────────────────
# Store the DB admin password and a generated session secret.
# mTLS certificates must be imported separately (see README.md).

resource "azurerm_key_vault_secret" "db_password" {
  name         = "db-password"
  value        = var.db_admin_password
  key_vault_id = azurerm_key_vault.main.id

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

resource "random_password" "session_secret" {
  length           = 64
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "azurerm_key_vault_secret" "session_secret" {
  name         = "session-secret"
  value        = random_password.session_secret.result
  key_vault_id = azurerm_key_vault.main.id

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

# ─── Placeholder Secrets for Certificates ────────────────────────────────────
# These placeholders are created so that the Key Vault structure is complete.
# Replace them by running the cert-generation script documented in README.md.

resource "azurerm_key_vault_secret" "ca_cert" {
  name         = "ca-cert"
  value        = "PLACEHOLDER — replace with PEM-encoded CA certificate"
  key_vault_id = azurerm_key_vault.main.id

  lifecycle {
    ignore_changes = [value]
  }

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

resource "azurerm_key_vault_secret" "controller_cert" {
  name         = "controller-cert"
  value        = "PLACEHOLDER — replace with PEM-encoded controller certificate"
  key_vault_id = azurerm_key_vault.main.id

  lifecycle {
    ignore_changes = [value]
  }

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

resource "azurerm_key_vault_secret" "controller_key" {
  name         = "controller-key"
  value        = "PLACEHOLDER — replace with PEM-encoded controller private key"
  key_vault_id = azurerm_key_vault.main.id

  lifecycle {
    ignore_changes = [value]
  }

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

resource "azurerm_key_vault_secret" "agent_cert" {
  name         = "agent-cert"
  value        = "PLACEHOLDER — replace with PEM-encoded agent certificate"
  key_vault_id = azurerm_key_vault.main.id

  lifecycle {
    ignore_changes = [value]
  }

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}

resource "azurerm_key_vault_secret" "agent_key" {
  name         = "agent-key"
  value        = "PLACEHOLDER — replace with PEM-encoded agent private key"
  key_vault_id = azurerm_key_vault.main.id

  lifecycle {
    ignore_changes = [value]
  }

  tags = local.common_tags

  depends_on = [azurerm_key_vault_access_policy.terraform_operator]
}
