# ── Load Balancers — HA mode only ──────────────────────────────────────────────
# All resources in this file use count = var.enable_ha ? 1 : 0 so they are
# created only when enable_ha = true.  In standard mode agents connect directly
# to the single controller instance; the ASG provides replacement-only HA.

# ── Application Load Balancer — HTTP / Web UI ──────────────────────────────────

resource "aws_lb" "http" {
  count = var.enable_ha ? 1 : 0

  name               = "encodeswarmr-${var.environment}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection = var.environment == "prod" ? true : false

  tags = {
    Name = "encodeswarmr-${var.environment}-alb"
  }
}

# ── ALB HTTP Target Group ──────────────────────────────────────────────────────

resource "aws_lb_target_group" "http" {
  count = var.enable_ha ? 1 : 0

  name     = "encodeswarmr-${var.environment}-http-tg"
  port     = var.controller_http_port
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    path                = "/health"
    port                = var.controller_http_port
    protocol            = "HTTP"
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    matcher             = "200"
  }

  tags = {
    Name = "encodeswarmr-${var.environment}-http-tg"
  }
}

# ── ALB Listener — HTTP (redirects to HTTPS if cert provided, else forwards) ──

resource "aws_lb_listener" "http" {
  count = var.enable_ha ? 1 : 0

  load_balancer_arn = aws_lb.http[0].arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.http[0].arn
  }
}

# ── Network Load Balancer — gRPC TCP passthrough ───────────────────────────────
# NLB is required for gRPC because it operates at Layer 4 (TCP) and preserves
# the HTTP/2 framing that gRPC depends on.  The ALB does not support arbitrary
# TCP passthrough for mTLS without terminating TLS.

resource "aws_lb" "grpc" {
  count = var.enable_ha ? 1 : 0

  name               = "encodeswarmr-${var.environment}-nlb"
  internal           = false
  load_balancer_type = "network"
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection       = var.environment == "prod" ? true : false
  enable_cross_zone_load_balancing = true

  tags = {
    Name = "encodeswarmr-${var.environment}-nlb"
  }
}

# ── NLB gRPC Target Group ──────────────────────────────────────────────────────

resource "aws_lb_target_group" "grpc" {
  count = var.enable_ha ? 1 : 0

  name     = "encodeswarmr-${var.environment}-grpc-tg"
  port     = var.controller_grpc_port
  protocol = "TCP"
  vpc_id   = aws_vpc.main.id

  health_check {
    port                = var.controller_grpc_port
    protocol            = "TCP"
    healthy_threshold   = 2
    unhealthy_threshold = 2
    interval            = 30
  }

  tags = {
    Name = "encodeswarmr-${var.environment}-grpc-tg"
  }
}

# ── NLB gRPC Listener ─────────────────────────────────────────────────────────

resource "aws_lb_listener" "grpc" {
  count = var.enable_ha ? 1 : 0

  load_balancer_arn = aws_lb.grpc[0].arn
  port              = var.controller_grpc_port
  protocol          = "TCP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.grpc[0].arn
  }
}
