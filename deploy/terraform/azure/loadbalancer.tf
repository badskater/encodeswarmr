# ─── Load Balancer (HA mode only) ────────────────────────────────────────────
# Standard SKU Azure Load Balancer fronting 2 controller VMs.
# Routes HTTP :8080 (web UI/API) and gRPC/mTLS :9443 (agents).

resource "azurerm_lb" "controller" {
  count               = var.enable_ha ? 1 : 0
  name                = "${local.name_prefix}-controller-lb"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                 = "Standard"

  frontend_ip_configuration {
    name                 = "controller-frontend"
    public_ip_address_id = azurerm_public_ip.lb[0].id
  }

  tags = local.common_tags
}

# ─── Backend Address Pool ─────────────────────────────────────────────────────

resource "azurerm_lb_backend_address_pool" "controller" {
  count           = var.enable_ha ? 1 : 0
  name            = "controller-backend-pool"
  loadbalancer_id = azurerm_lb.controller[0].id
}

# ─── Health Probe — HTTP /health ──────────────────────────────────────────────

resource "azurerm_lb_probe" "http_health" {
  count               = var.enable_ha ? 1 : 0
  name                = "http-health-probe"
  loadbalancer_id     = azurerm_lb.controller[0].id
  protocol            = "Http"
  port                = 8080
  request_path        = "/health"
  interval_in_seconds = 15
  number_of_probes    = 3
}

# ─── Health Probe — TCP :9443 ────────────────────────────────────────────────

resource "azurerm_lb_probe" "grpc_health" {
  count               = var.enable_ha ? 1 : 0
  name                = "grpc-health-probe"
  loadbalancer_id     = azurerm_lb.controller[0].id
  protocol            = "Tcp"
  port                = 9443
  interval_in_seconds = 15
  number_of_probes    = 3
}

# ─── LB Rules ─────────────────────────────────────────────────────────────────

# HTTP :8080
resource "azurerm_lb_rule" "http" {
  count                          = var.enable_ha ? 1 : 0
  name                           = "http-8080"
  loadbalancer_id                = azurerm_lb.controller[0].id
  protocol                       = "Tcp"
  frontend_port                  = 8080
  backend_port                   = 8080
  frontend_ip_configuration_name = "controller-frontend"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.controller[0].id]
  probe_id                       = azurerm_lb_probe.http_health[0].id
  idle_timeout_in_minutes        = 4
  enable_floating_ip             = false
  disable_outbound_snat          = true
}

# gRPC mTLS :9443
resource "azurerm_lb_rule" "grpc" {
  count                          = var.enable_ha ? 1 : 0
  name                           = "grpc-9443"
  loadbalancer_id                = azurerm_lb.controller[0].id
  protocol                       = "Tcp"
  frontend_port                  = 9443
  backend_port                   = 9443
  frontend_ip_configuration_name = "controller-frontend"
  backend_address_pool_ids       = [azurerm_lb_backend_address_pool.controller[0].id]
  probe_id                       = azurerm_lb_probe.grpc_health[0].id
  idle_timeout_in_minutes        = 30
  enable_floating_ip             = false
  disable_outbound_snat          = true
}

# ─── Outbound Rule for Controllers ────────────────────────────────────────────
# Required when disable_outbound_snat = true on LB rules.

resource "azurerm_lb_outbound_rule" "controller" {
  count                   = var.enable_ha ? 1 : 0
  name                    = "controller-outbound"
  loadbalancer_id         = azurerm_lb.controller[0].id
  protocol                = "All"
  backend_address_pool_id = azurerm_lb_backend_address_pool.controller[0].id

  frontend_ip_configuration {
    name = "controller-frontend"
  }
}
