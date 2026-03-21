# ── RDS PostgreSQL 16 ──────────────────────────────────────────────────────────

resource "aws_db_instance" "main" {
  identifier = "encodeswarmr-${var.environment}-postgres"

  # Engine
  engine               = "postgres"
  engine_version       = "16"
  instance_class       = var.db_instance_class
  parameter_group_name = aws_db_parameter_group.main.name

  # Storage — gp3 for consistent IOPS baseline
  storage_type          = "gp3"
  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_max_allocated_storage
  storage_encrypted     = true

  # Credentials
  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  # Networking
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.database.id]
  publicly_accessible    = false

  # HA — enable Multi-AZ when enable_ha = true; var.db_multi_az allows manual override
  multi_az = var.enable_ha ? true : var.db_multi_az

  # Backups and maintenance
  backup_retention_period   = var.db_backup_retention_days
  backup_window             = "03:00-04:00"
  maintenance_window        = "Mon:04:00-Mon:05:00"
  auto_minor_version_upgrade = true
  deletion_protection       = var.environment == "prod" ? true : false

  # Monitoring
  performance_insights_enabled          = true
  performance_insights_retention_period = 7
  monitoring_interval                   = 60
  monitoring_role_arn                   = aws_iam_role.rds_monitoring.arn
  enabled_cloudwatch_logs_exports       = ["postgresql", "upgrade"]

  # Snapshot on delete for non-dev environments
  skip_final_snapshot       = var.environment == "dev" ? true : false
  final_snapshot_identifier = var.environment != "dev" ? "encodeswarmr-${var.environment}-final-snapshot" : null

  tags = {
    Name = "encodeswarmr-${var.environment}-postgres"
  }
}

# ── Parameter Group ────────────────────────────────────────────────────────────

resource "aws_db_parameter_group" "main" {
  name   = "encodeswarmr-${var.environment}-pg16"
  family = "postgres16"

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }

  parameter {
    name  = "log_duration"
    value = "0"
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "1000" # Log queries slower than 1 second
  }

  parameter {
    name  = "max_connections"
    value = "200"
  }

  tags = {
    Name = "encodeswarmr-${var.environment}-pg16"
  }
}

# ── IAM Role for Enhanced Monitoring ──────────────────────────────────────────

resource "aws_iam_role" "rds_monitoring" {
  name = "encodeswarmr-${var.environment}-rds-monitoring"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "encodeswarmr-${var.environment}-rds-monitoring"
  }
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}
