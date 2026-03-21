# ---------------------------------------------------------------------------
# Filestore — NFS shared storage
#
# Replaces the NAS/SAN used in on-premises deployments.
# Both controller and agent VMs mount the NFS shares at:
#   /mnt/nas/media    — source media files
#   /mnt/nas/encodes  — encoded output
#   /mnt/nas/temp     — temporary working files
#
# Tier selection (from var.filestore_tier):
#   BASIC_HDD  — minimum 1 TiB, ~$0.20/GiB/month, suitable for most workloads.
#   BASIC_SSD  — minimum 2.5 TiB, ~$0.35/GiB/month, lower latency.
# ---------------------------------------------------------------------------

resource "google_filestore_instance" "media" {
  name     = "${local.name_prefix}-filestore"
  tier     = var.filestore_tier
  location = var.zone
  project  = var.project_id

  description = "Shared NFS storage for distributed-encoder media, encodes, and temp files."

  file_shares {
    name        = "media"
    capacity_gb = var.filestore_capacity_gb
  }

  networks {
    network      = google_compute_network.main.name
    modes        = ["MODE_IPV4"]
    connect_mode = "DIRECT_PEERING"
  }

  labels = local.common_labels
}

# ---------------------------------------------------------------------------
# Outputs used by startup scripts to construct NFS mount commands.
# ---------------------------------------------------------------------------

locals {
  filestore_ip    = google_filestore_instance.media.networks[0].ip_addresses[0]
  filestore_share = "/${google_filestore_instance.media.file_shares[0].name}"

  # NFS mount commands injected into VM startup scripts.
  nfs_mount_script = <<-EOT
    apt-get install -y nfs-common
    mkdir -p /mnt/nas/media /mnt/nas/encodes /mnt/nas/temp

    # Mount the Filestore share and create subdirectories on first boot.
    mount -t nfs ${local.filestore_ip}:${local.filestore_share} /mnt/nas/media
    mkdir -p /mnt/nas/media/encodes /mnt/nas/media/temp

    # Bind-mount subdirectories to the expected paths.
    mount --bind /mnt/nas/media/encodes /mnt/nas/encodes
    mount --bind /mnt/nas/media/temp    /mnt/nas/temp

    # Persist mounts across reboots.
    grep -q "${local.filestore_ip}" /etc/fstab || cat >> /etc/fstab <<FSTAB
${local.filestore_ip}:${local.filestore_share} /mnt/nas/media nfs defaults,_netdev 0 0
/mnt/nas/media/encodes /mnt/nas/encodes none bind 0 0
/mnt/nas/media/temp    /mnt/nas/temp    none bind 0 0
FSTAB
  EOT
}
