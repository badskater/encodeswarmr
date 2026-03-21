# ─── Agent Cloud-Init ─────────────────────────────────────────────────────────

locals {
  # In HA mode, agents connect to the load balancer IP; in standard mode, to the
  # first (only) controller's public IP.
  controller_address = var.enable_ha ? (
    "${azurerm_public_ip.lb[0].fqdn}:9443"
    ) : (
    "${azurerm_public_ip.controller[0].ip_address}:9443"
  )

  agent_cloud_init = templatefile("${path.module}/templates/agent-cloud-init.yaml.tpl", {
    controller_address  = local.controller_address
    distencoder_version = var.distencoder_version
    storage_account     = azurerm_storage_account.nas.name
    key_vault_name      = azurerm_key_vault.main.name
    nfs_enabled         = var.storage_account_tier == "Premium"
  })
}

# ─── Agent VMSS ───────────────────────────────────────────────────────────────

resource "azurerm_linux_virtual_machine_scale_set" "agents" {
  name                = "${local.name_prefix}-agents-vmss"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  sku                 = var.agent_vm_size
  instances           = var.agent_count
  admin_username      = "distencoder"

  admin_ssh_key {
    username   = "distencoder"
    public_key = file(var.ssh_public_key_path)
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }

  os_disk {
    storage_account_type = "Premium_LRS"
    caching              = "ReadWrite"
    disk_size_gb         = 128
  }

  # Additional data disk for work/temp files during encoding
  data_disk {
    storage_account_type = "Premium_LRS"
    caching              = "ReadWrite"
    disk_size_gb         = 512
    lun                  = 0
    create_option        = "Empty"
  }

  network_interface {
    name    = "agent-nic"
    primary = true

    network_security_group_id = azurerm_network_security_group.agent.id

    ip_configuration {
      name      = "internal"
      primary   = true
      subnet_id = azurerm_subnet.agent.id
    }
  }

  custom_data = base64encode(local.agent_cloud_init)

  identity {
    type = "SystemAssigned"
  }

  # Spread instances across zones in HA mode
  zones = var.enable_ha ? ["1", "2", "3"] : []
  zone_balance = var.enable_ha ? true : false

  # Upgrade policy: Rolling to avoid simultaneous agent downtime
  upgrade_mode = "Rolling"

  rolling_upgrade_policy {
    max_batch_instance_percent              = 25
    max_unhealthy_instance_percent          = 50
    max_unhealthy_upgraded_instance_percent = 25
    pause_time_between_batches              = "PT1M"
    prioritize_unhealthy_instances_enabled  = true
  }

  # Health extension to support rolling upgrades
  extension {
    name                       = "HealthExtension"
    publisher                  = "Microsoft.ManagedServices"
    type                       = "ApplicationHealthLinux"
    type_handler_version       = "1.0"
    auto_upgrade_minor_version = true

    settings = jsonencode({
      protocol    = "tcp"
      port        = 9443
      requestPath = ""
    })
  }

  # Automatic repair if an instance becomes unhealthy
  automatic_instance_repair {
    enabled      = true
    grace_period = "PT30M"
  }

  # Scale-in policy: remove newest instances first
  scale_in {
    rule                   = "NewestVM"
    force_deletion_enabled = false
  }

  tags = merge(local.common_tags, {
    role = "agent"
  })

  depends_on = [
    azurerm_linux_virtual_machine.controller,
    azurerm_key_vault_access_policy.agent,
  ]
}

# ─── Agent VMSS Key Vault Access ─────────────────────────────────────────────
# Agents need read access to pull mTLS certs from Key Vault.
# VMSS managed identity principal_id is available after creation.

resource "azurerm_key_vault_access_policy" "agent" {
  key_vault_id = azurerm_key_vault.main.id
  tenant_id    = data.azurerm_client_config.current.tenant_id
  object_id    = azurerm_linux_virtual_machine_scale_set.agents.identity[0].principal_id

  secret_permissions = [
    "Get",
    "List",
  ]

  certificate_permissions = [
    "Get",
    "List",
  ]
}
