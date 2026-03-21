# ── mTLS Certificate Authority ─────────────────────────────────────────────────
# Self-signed CA used to issue controller and agent leaf certificates for mTLS.
# All keys and certs are stored in SSM Parameter Store as SecureString so
# instances can fetch them at boot without baking secrets into AMIs or user-data.

resource "tls_private_key" "ca" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "tls_self_signed_cert" "ca" {
  private_key_pem = tls_private_key.ca.private_key_pem

  subject {
    common_name  = "distributed-encoder-${var.environment}-ca"
    organization = "distributed-encoder"
  }

  validity_period_hours = 87600 # 10 years

  is_ca_certificate = true

  allowed_uses = [
    "cert_signing",
    "crl_signing",
    "key_encipherment",
    "digital_signature",
  ]
}

# ── Controller Certificate ─────────────────────────────────────────────────────

resource "tls_private_key" "controller" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "controller" {
  private_key_pem = tls_private_key.controller.private_key_pem

  subject {
    common_name  = "controller.distencoder.internal"
    organization = "distributed-encoder"
  }

  dns_names = [
    "controller.distencoder.internal",
    "localhost",
  ]
}

resource "tls_locally_signed_cert" "controller" {
  cert_request_pem   = tls_cert_request.controller.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 43800 # 5 years

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
    "client_auth",
  ]
}

# ── Agent Certificate ──────────────────────────────────────────────────────────

resource "tls_private_key" "agent" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "agent" {
  private_key_pem = tls_private_key.agent.private_key_pem

  subject {
    common_name  = "agent.distencoder.internal"
    organization = "distributed-encoder"
  }
}

resource "tls_locally_signed_cert" "agent" {
  cert_request_pem   = tls_cert_request.agent.cert_request_pem
  ca_private_key_pem = tls_private_key.ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.ca.cert_pem

  validity_period_hours = 43800 # 5 years

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "client_auth",
  ]
}

# ── SSM Parameter Store — CA ───────────────────────────────────────────────────

resource "aws_ssm_parameter" "ca_cert" {
  name  = "/distencoder/${var.environment}/certs/ca.crt"
  type  = "String"
  value = tls_self_signed_cert.ca.cert_pem

  tags = {
    Name = "distencoder-${var.environment}-ca-cert"
  }
}

resource "aws_ssm_parameter" "ca_key" {
  name  = "/distencoder/${var.environment}/certs/ca.key"
  type  = "SecureString"
  value = tls_private_key.ca.private_key_pem

  tags = {
    Name = "distencoder-${var.environment}-ca-key"
  }
}

# ── SSM Parameter Store — Controller ──────────────────────────────────────────

resource "aws_ssm_parameter" "controller_cert" {
  name  = "/distencoder/${var.environment}/certs/controller.crt"
  type  = "String"
  value = tls_locally_signed_cert.controller.cert_pem

  tags = {
    Name = "distencoder-${var.environment}-controller-cert"
  }
}

resource "aws_ssm_parameter" "controller_key" {
  name  = "/distencoder/${var.environment}/certs/controller.key"
  type  = "SecureString"
  value = tls_private_key.controller.private_key_pem

  tags = {
    Name = "distencoder-${var.environment}-controller-key"
  }
}

# ── SSM Parameter Store — Agent ────────────────────────────────────────────────

resource "aws_ssm_parameter" "agent_cert" {
  name  = "/distencoder/${var.environment}/certs/agent.crt"
  type  = "String"
  value = tls_locally_signed_cert.agent.cert_pem

  tags = {
    Name = "distencoder-${var.environment}-agent-cert"
  }
}

resource "aws_ssm_parameter" "agent_key" {
  name  = "/distencoder/${var.environment}/certs/agent.key"
  type  = "SecureString"
  value = tls_private_key.agent.private_key_pem

  tags = {
    Name = "distencoder-${var.environment}-agent-key"
  }
}

# ── SSM Parameter Store — Application secrets ──────────────────────────────────

resource "aws_ssm_parameter" "db_password" {
  name  = "/distencoder/${var.environment}/db/password"
  type  = "SecureString"
  value = var.db_password

  tags = {
    Name = "distencoder-${var.environment}-db-password"
  }
}

resource "aws_ssm_parameter" "session_secret" {
  name  = "/distencoder/${var.environment}/controller/session_secret"
  type  = "SecureString"
  value = var.controller_session_secret

  tags = {
    Name = "distencoder-${var.environment}-session-secret"
  }
}
