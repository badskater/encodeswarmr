# ─── Controller NSG ───────────────────────────────────────────────────────────
# Allows: HTTP :8080 (web UI + API), gRPC :9443, SSH :22 (restricted)

resource "azurerm_network_security_group" "controller" {
  name                = "${local.name_prefix}-controller-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  # HTTP — web UI and REST API
  security_rule {
    name                       = "allow-http-8080"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "8080"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  # gRPC — agent-to-controller mTLS
  security_rule {
    name                       = "allow-grpc-9443"
    priority                   = 110
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "9443"
    source_address_prefix      = var.agent_subnet_prefix
    destination_address_prefix = "*"
  }

  # SSH — restricted to allowed CIDRs
  security_rule {
    name                   = "allow-ssh-22"
    priority               = 120
    direction              = "Inbound"
    access                 = length(var.allowed_ssh_cidrs) > 0 ? "Allow" : "Deny"
    protocol               = "Tcp"
    source_port_range      = "*"
    destination_port_range = "22"
    source_address_prefixes = length(var.allowed_ssh_cidrs) > 0 ? var.allowed_ssh_cidrs : ["0.0.0.0/0"]
    destination_address_prefix = "*"
  }

  # Deny all other inbound
  security_rule {
    name                       = "deny-all-inbound"
    priority                   = 4096
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  tags = local.common_tags
}

# ─── Agent NSG ────────────────────────────────────────────────────────────────
# Allows: SSH :22 (restricted), outbound :9443 to controller subnet

resource "azurerm_network_security_group" "agent" {
  name                = "${local.name_prefix}-agent-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  # SSH — restricted to allowed CIDRs
  security_rule {
    name                   = "allow-ssh-22"
    priority               = 100
    direction              = "Inbound"
    access                 = length(var.allowed_ssh_cidrs) > 0 ? "Allow" : "Deny"
    protocol               = "Tcp"
    source_port_range      = "*"
    destination_port_range = "22"
    source_address_prefixes = length(var.allowed_ssh_cidrs) > 0 ? var.allowed_ssh_cidrs : ["0.0.0.0/0"]
    destination_address_prefix = "*"
  }

  # Deny all other inbound
  security_rule {
    name                       = "deny-all-inbound"
    priority                   = 4096
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  # Allow outbound gRPC to controller subnet
  security_rule {
    name                       = "allow-outbound-grpc-9443"
    priority                   = 100
    direction                  = "Outbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "9443"
    source_address_prefix      = "*"
    destination_address_prefix = var.controller_subnet_prefix
  }

  # Allow outbound HTTPS (package installs, Key Vault, Azure Storage)
  security_rule {
    name                       = "allow-outbound-https"
    priority                   = 110
    direction                  = "Outbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "*"
    destination_address_prefix = "Internet"
  }

  tags = local.common_tags
}

# ─── Database NSG ─────────────────────────────────────────────────────────────
# Allows: PostgreSQL :5432 from controller subnet only

resource "azurerm_network_security_group" "database" {
  name                = "${local.name_prefix}-database-nsg"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name

  # PostgreSQL from controller subnet
  security_rule {
    name                       = "allow-postgres-from-controller"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "5432"
    source_address_prefix      = var.controller_subnet_prefix
    destination_address_prefix = "*"
  }

  # Deny all other inbound
  security_rule {
    name                       = "deny-all-inbound"
    priority                   = 4096
    direction                  = "Inbound"
    access                     = "Deny"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  tags = local.common_tags
}
