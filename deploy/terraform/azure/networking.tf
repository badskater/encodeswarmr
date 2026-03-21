# ─── Virtual Network ──────────────────────────────────────────────────────────

resource "azurerm_virtual_network" "main" {
  name                = "${local.name_prefix}-vnet"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  address_space       = [var.vnet_address_space]

  tags = local.common_tags
}

# ─── Controller Subnet ────────────────────────────────────────────────────────

resource "azurerm_subnet" "controller" {
  name                 = "controller-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.controller_subnet_prefix]
}

# ─── Agent Subnet ─────────────────────────────────────────────────────────────

resource "azurerm_subnet" "agent" {
  name                 = "agent-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.agent_subnet_prefix]
}

# ─── Database Subnet (delegated to PostgreSQL Flexible Server) ────────────────

resource "azurerm_subnet" "database" {
  name                 = "database-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.database_subnet_prefix]

  # Required delegation for PostgreSQL Flexible Server VNet integration
  delegation {
    name = "postgresql-delegation"
    service_delegation {
      name = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = [
        "Microsoft.Network/virtualNetworks/subnets/join/action",
      ]
    }
  }

  service_endpoints = ["Microsoft.Storage"]
}

# ─── Storage Private Endpoint Subnet ─────────────────────────────────────────

resource "azurerm_subnet" "storage" {
  name                 = "storage-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [var.storage_subnet_prefix]

  private_endpoint_network_policies = "Disabled"
}

# ─── NSG Associations ─────────────────────────────────────────────────────────

resource "azurerm_subnet_network_security_group_association" "controller" {
  subnet_id                 = azurerm_subnet.controller.id
  network_security_group_id = azurerm_network_security_group.controller.id
}

resource "azurerm_subnet_network_security_group_association" "agent" {
  subnet_id                 = azurerm_subnet.agent.id
  network_security_group_id = azurerm_network_security_group.agent.id
}

resource "azurerm_subnet_network_security_group_association" "database" {
  subnet_id                 = azurerm_subnet.database.id
  network_security_group_id = azurerm_network_security_group.database.id
}

# ─── Public IPs ───────────────────────────────────────────────────────────────

# Controller public IPs (used directly in standard mode; fronted by LB in HA mode)
resource "azurerm_public_ip" "controller" {
  count               = local.controller_count
  name                = "${local.name_prefix}-controller-pip-${count.index}"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  allocation_method   = "Static"
  sku                 = "Standard"
  zones               = var.enable_ha ? ["1", "2", "3"] : []

  tags = local.common_tags
}

# Load balancer public IP (HA only)
resource "azurerm_public_ip" "lb" {
  count               = var.enable_ha ? 1 : 0
  name                = "${local.name_prefix}-lb-pip"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  allocation_method   = "Static"
  sku                 = "Standard"
  zones               = ["1", "2", "3"]
  domain_name_label   = "${local.name_prefix}-lb"

  tags = local.common_tags
}
