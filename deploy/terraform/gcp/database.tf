# ---------------------------------------------------------------------------
# Cloud SQL — PostgreSQL 16
#
# Standard deployment : ZONAL availability (single instance).
# HA deployment       : REGIONAL availability + point-in-time recovery.
# Both configurations use private IP only (no public IP).
# ---------------------------------------------------------------------------

resource "google_sql_database_instance" "main" {
  name             = "${local.name_prefix}-pg-${random_id.suffix.hex}"
  database_version = "POSTGRES_16"
  region           = var.region
  project          = var.project_id

  # Prevent accidental deletion of the database.
  deletion_protection = true

  settings {
    tier              = var.db_tier
    availability_type = var.enable_ha ? "REGIONAL" : "ZONAL"
    disk_autoresize   = true
    disk_size         = 20
    disk_type         = "PD_SSD"

    user_labels = local.common_labels

    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.main.id
      require_ssl     = true
    }

    backup_configuration {
      enabled                        = true
      start_time                     = "03:00"
      point_in_time_recovery_enabled = var.enable_ha
      transaction_log_retention_days = var.enable_ha ? 7 : 1
      backup_retention_settings {
        retained_backups = var.enable_ha ? 14 : 7
        retention_unit   = "COUNT"
      }
    }

    maintenance_window {
      day          = 7 # Sunday
      hour         = 4
      update_track = "stable"
    }

    database_flags {
      name  = "max_connections"
      value = "100"
    }

    database_flags {
      name  = "log_min_duration_statement"
      value = "1000" # log queries slower than 1 s
    }

    insights_config {
      query_insights_enabled  = true
      query_string_length     = 1024
      record_application_tags = true
      record_client_address   = false
    }
  }

  depends_on = [google_service_networking_connection.private_vpc_connection]
}

resource "google_sql_database" "encodeswarmr" {
  name     = var.db_name
  instance = google_sql_database_instance.main.name
  project  = var.project_id
}

resource "google_sql_user" "encodeswarmr" {
  name     = var.db_user
  instance = google_sql_database_instance.main.name
  password = var.db_password
  project  = var.project_id
}
