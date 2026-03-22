# ─── Controller Cloud-Init ────────────────────────────────────────────────────
# Rendered per-instance so each gets its index for naming certs.

locals {
  controller_cloud_init = templatefile("${path.module}/templates/controller-cloud-init.yaml.tpl", {
    docker_image        = var.controller_docker_image
    encodeswarmr_version = var.encodeswarmr_version
    storage_account     = azurerm_storage_account.nas.name
    db_host             = azurerm_postgresql_flexible_server.main.fqdn
    db_name             = azurerm_postgresql_flexible_server_database.encodeswarmr.name
    db_admin_login      = var.db_admin_login
    key_vault_name      = azurerm_key_vault.main.name
    nfs_enabled         = var.storage_account_tier == "Premium"
  })
}

# ─── Controller NICs ──────────────────────────────────────────────────────────

resource "azurerm_network_interface" "controller" {
  count               = local.controller_count
  name                = "${local.name_prefix}-controller-nic-${count.index}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.controller.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.controller[count.index].id
  }

  tags = local.common_tags
}

# ─── Controller VMs ───────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine" "controller" {
  count               = local.controller_count
  name                = "${local.name_prefix}-controller-${count.index}"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  size                = var.controller_vm_size

  # Spread across zones in HA mode
  zone = var.enable_ha ? tostring(count.index + 1) : null

  network_interface_ids = [azurerm_network_interface.controller[count.index].id]

  admin_username = "encodeswarmr"
  admin_ssh_key {
    username   = "encodeswarmr"
    public_key = file(var.ssh_public_key_path)
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS"
    disk_size_gb         = 64
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }

  custom_data = base64encode(local.controller_cloud_init)

  identity {
    type = "SystemAssigned"
  }

  tags = merge(local.common_tags, {
    role  = "controller"
    index = tostring(count.index)
  })
}

# ─── Controller Key Vault Access ──────────────────────────────────────────────
# Grant each controller VM's managed identity access to Key Vault secrets.

resource "azurerm_key_vault_access_policy" "controller" {
  count        = local.controller_count
  key_vault_id = azurerm_key_vault.main.id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = azurerm_linux_virtual_machine.controller[count.index].identity[0].principal_id

  secret_permissions = [
    "Get",
    "List",
    "Set",
  ]

  certificate_permissions = [
    "Get",
    "List",
    "Import",
    "Create",
  ]
}

# ─── Controller → LB Backend Pool Association (HA only) ──────────────────────

resource "azurerm_network_interface_backend_address_pool_association" "controller" {
  count                   = var.enable_ha ? local.controller_count : 0
  network_interface_id    = azurerm_network_interface.controller[count.index].id
  ip_configuration_name   = "internal"
  backend_address_pool_id = azurerm_lb_backend_address_pool.controller[0].id
}
