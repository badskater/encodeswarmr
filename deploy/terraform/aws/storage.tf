# ── EFS File System ────────────────────────────────────────────────────────────
# EFS replaces the on-premise NAS for shared media, encode output, and temp.
# Controller and agents mount EFS under /mnt/nas/* to match the expected
# path_mappings configured in controller.yaml.

resource "aws_efs_file_system" "main" {
  creation_token   = "distencoder-${var.environment}-efs"
  performance_mode = var.efs_performance_mode
  throughput_mode  = var.efs_throughput_mode
  encrypted        = true

  lifecycle_policy {
    transition_to_ia = "AFTER_30_DAYS"
  }

  lifecycle_policy {
    transition_to_primary_storage_class = "AFTER_1_ACCESS"
  }

  tags = {
    Name = "distencoder-${var.environment}-efs"
  }
}

# ── Mount Targets (one per private subnet / AZ) ────────────────────────────────

resource "aws_efs_mount_target" "main" {
  count = 2

  file_system_id  = aws_efs_file_system.main.id
  subnet_id       = aws_subnet.private[count.index].id
  security_groups = [aws_security_group.efs.id]
}

# ── Access Points ──────────────────────────────────────────────────────────────
# Three access points mirror the on-premise NAS exports used by the application:
#   /media   — source media files (read/write for controller and agents)
#   /encodes — encode output destination
#   /temp    — temporary working files

resource "aws_efs_access_point" "media" {
  file_system_id = aws_efs_file_system.main.id

  posix_user {
    uid = 1000
    gid = 1000
  }

  root_directory {
    path = "/media"
    creation_info {
      owner_uid   = 1000
      owner_gid   = 1000
      permissions = "0755"
    }
  }

  tags = {
    Name = "distencoder-${var.environment}-efs-media"
  }
}

resource "aws_efs_access_point" "encodes" {
  file_system_id = aws_efs_file_system.main.id

  posix_user {
    uid = 1000
    gid = 1000
  }

  root_directory {
    path = "/encodes"
    creation_info {
      owner_uid   = 1000
      owner_gid   = 1000
      permissions = "0755"
    }
  }

  tags = {
    Name = "distencoder-${var.environment}-efs-encodes"
  }
}

resource "aws_efs_access_point" "temp" {
  file_system_id = aws_efs_file_system.main.id

  posix_user {
    uid = 1000
    gid = 1000
  }

  root_directory {
    path = "/temp"
    creation_info {
      owner_uid   = 1000
      owner_gid   = 1000
      permissions = "0777"
    }
  }

  tags = {
    Name = "distencoder-${var.environment}-efs-temp"
  }
}

# ── EFS Backup Policy ──────────────────────────────────────────────────────────

resource "aws_efs_backup_policy" "main" {
  file_system_id = aws_efs_file_system.main.id

  backup_policy {
    status = var.environment == "prod" ? "ENABLED" : "DISABLED"
  }
}
