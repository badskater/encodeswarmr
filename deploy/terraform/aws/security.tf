# ── ALB Security Group ─────────────────────────────────────────────────────────
# Only created in HA mode (loadbalancer.tf uses count = var.enable_ha ? 1 : 0).
# We still define the SG unconditionally so the controller SG can reference it.

resource "aws_security_group" "alb" {
  name        = "distencoder-${var.environment}-alb-sg"
  description = "Allow HTTP and HTTPS inbound to the Application Load Balancer."
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from anywhere"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS from anywhere"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "distencoder-${var.environment}-alb-sg"
  }
}

# ── Controller Security Group ──────────────────────────────────────────────────

resource "aws_security_group" "controller" {
  name        = "distencoder-${var.environment}-controller-sg"
  description = "Allow HTTP REST/UI, gRPC mTLS, and SSH inbound to controller instances."
  vpc_id      = aws_vpc.main.id

  # HTTP — from ALB in HA mode, or open in standard mode
  ingress {
    description     = "HTTP from ALB"
    from_port       = var.controller_http_port
    to_port         = var.controller_http_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  ingress {
    description = "HTTP direct access (standard / non-HA)"
    from_port   = var.controller_http_port
    to_port     = var.controller_http_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # gRPC mTLS — agents connect here
  ingress {
    description = "gRPC mTLS from agent SG"
    from_port   = var.controller_grpc_port
    to_port     = var.controller_grpc_port
    protocol    = "tcp"
    self        = true
  }

  ingress {
    description     = "gRPC mTLS from agents"
    from_port       = var.controller_grpc_port
    to_port         = var.controller_grpc_port
    protocol        = "tcp"
    security_groups = [aws_security_group.agent.id]
  }

  # SSH
  dynamic "ingress" {
    for_each = length(var.allowed_ssh_cidrs) > 0 ? [1] : []
    content {
      description = "SSH access"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.allowed_ssh_cidrs
    }
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "distencoder-${var.environment}-controller-sg"
  }
}

# ── Agent Security Group ───────────────────────────────────────────────────────

resource "aws_security_group" "agent" {
  name        = "distencoder-${var.environment}-agent-sg"
  description = "Encoding agent instances. Allow SSH inbound; gRPC outbound to controller."
  vpc_id      = aws_vpc.main.id

  dynamic "ingress" {
    for_each = length(var.allowed_ssh_cidrs) > 0 ? [1] : []
    content {
      description = "SSH access"
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = var.allowed_ssh_cidrs
    }
  }

  # Outbound gRPC to controller
  egress {
    description     = "gRPC mTLS to controller"
    from_port       = var.controller_grpc_port
    to_port         = var.controller_grpc_port
    protocol        = "tcp"
    security_groups = [aws_security_group.controller.id]
  }

  # General outbound (package installs, GitHub releases, NTP, etc.)
  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "distencoder-${var.environment}-agent-sg"
  }
}

# ── Database Security Group ────────────────────────────────────────────────────

resource "aws_security_group" "database" {
  name        = "distencoder-${var.environment}-db-sg"
  description = "Allow PostgreSQL connections from controller instances only."
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "PostgreSQL from controller"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.controller.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "distencoder-${var.environment}-db-sg"
  }
}

# ── EFS Security Group ─────────────────────────────────────────────────────────

resource "aws_security_group" "efs" {
  name        = "distencoder-${var.environment}-efs-sg"
  description = "Allow NFS traffic from controller and agent instances."
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "NFS from controller"
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.controller.id]
  }

  ingress {
    description     = "NFS from agents"
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.agent.id]
  }

  egress {
    description = "Allow all outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "distencoder-${var.environment}-efs-sg"
  }
}
