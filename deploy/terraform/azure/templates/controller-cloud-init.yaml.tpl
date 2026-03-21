#cloud-config
# Controller cloud-init — installs Docker, mounts Azure Files, starts controller container.
# Secrets are fetched from Azure Key Vault using the VM's managed identity.

package_update: true
package_upgrade: true

packages:
  - docker.io
  - docker-compose-v2
  - curl
  - jq
  - nfs-common
  - cifs-utils
  - azure-cli

write_files:
  - path: /etc/distributed-encoder/controller.yaml
    permissions: "0640"
    owner: root:docker
    content: |
      server:
        host: "0.0.0.0"
        port: 8080
        read_timeout: 30s
        write_timeout: 30s

      database:
        url: "postgres://${db_admin_login}:$${DB_PASSWORD}@${db_host}:5432/${db_name}?sslmode=require"
        max_conns: 25
        min_conns: 5
        max_conn_lifetime: 1h
        migrations_path: "/usr/share/distributed-encoder/migrations"

      grpc:
        host: "0.0.0.0"
        port: 9443
        tls:
          cert: "/etc/distributed-encoder/certs/controller.crt"
          key:  "/etc/distributed-encoder/certs/controller.key"
          ca:   "/etc/distributed-encoder/certs/ca.crt"

      tls:
        cert: "/etc/distributed-encoder/certs/controller.crt"
        key:  "/etc/distributed-encoder/certs/controller.key"
        ca:   "/etc/distributed-encoder/certs/ca.crt"

      logging:
        level: info
        format: json
        task_log_retention: 720h
        task_log_cleanup_interval: 6h

      agent:
        auto_approve: false
        heartbeat_timeout: 90s
        dispatch_interval: 10s
        stale_threshold: 5m

      engine:
        tick_interval: 10s
        stale_threshold: 5m

      analysis:
        ffmpeg_bin:  "/usr/bin/ffmpeg"
        ffprobe_bin: "/usr/bin/ffprobe"
        concurrency: 2

        path_mappings:
          - name:    "NAS media"
            windows: "\\\\azurefile\\media"
            linux:   "/mnt/nas/media"
          - name:    "NAS encodes"
            windows: "\\\\azurefile\\encodes"
            linux:   "/mnt/nas/encodes"
          - name:    "NAS temp"
            windows: "\\\\azurefile\\temp"
            linux:   "/mnt/nas/temp"

  - path: /etc/distributed-encoder/docker-compose.yml
    permissions: "0640"
    owner: root:docker
    content: |
      version: "3.9"
      services:
        controller:
          image: ${docker_image}
          restart: unless-stopped
          network_mode: host
          volumes:
            - /etc/distributed-encoder:/etc/distributed-encoder:ro
            - /mnt/nas:/mnt/nas
          env_file:
            - /etc/distributed-encoder/.env
          logging:
            driver: journald
            options:
              tag: "distencoder-controller"

  - path: /usr/local/bin/fetch-kv-secrets.sh
    permissions: "0750"
    owner: root:root
    content: |
      #!/bin/bash
      # Fetch secrets from Azure Key Vault using the VM managed identity.
      set -euo pipefail

      KV_NAME="${key_vault_name}"
      TOKEN=$(curl -s -H "Metadata: true" \
        "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://vault.azure.net" \
        | jq -r '.access_token')

      get_secret() {
        local name="$1"
        curl -s -H "Authorization: Bearer $TOKEN" \
          "https://$KV_NAME.vault.azure.net/secrets/$name?api-version=7.4" \
          | jq -r '.value'
      }

      mkdir -p /etc/distributed-encoder/certs
      chmod 750 /etc/distributed-encoder/certs

      # Fetch mTLS certificates
      get_secret "controller-cert" > /etc/distributed-encoder/certs/controller.crt
      get_secret "controller-key"  > /etc/distributed-encoder/certs/controller.key
      get_secret "ca-cert"         > /etc/distributed-encoder/certs/ca.crt
      chmod 640 /etc/distributed-encoder/certs/*.crt /etc/distributed-encoder/certs/*.key

      # Fetch DB password and write .env
      DB_PASSWORD=$(get_secret "db-password")
      SESSION_SECRET=$(get_secret "session-secret")

      cat > /etc/distributed-encoder/.env <<EOF
      DB_PASSWORD=$${DB_PASSWORD}
      SESSION_SECRET=$${SESSION_SECRET}
      EOF
      chmod 640 /etc/distributed-encoder/.env

      echo "Secrets fetched successfully."

  - path: /usr/local/bin/mount-nas.sh
    permissions: "0750"
    owner: root:root
    content: |
      #!/bin/bash
      # Mount Azure File shares.
      set -euo pipefail

      STORAGE_ACCOUNT="${storage_account}"
      NFS_ENABLED="${nfs_enabled}"

      mkdir -p /mnt/nas/media /mnt/nas/encodes /mnt/nas/temp

      if [ "$NFS_ENABLED" = "true" ]; then
        # NFS mount (Premium tier)
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/media"   /mnt/nas/media   -o vers=4,minorversion=1,sec=sys
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/encodes" /mnt/nas/encodes -o vers=4,minorversion=1,sec=sys
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/temp"    /mnt/nas/temp    -o vers=4,minorversion=1,sec=sys
      else
        # SMB mount (Standard tier) — key fetched from Azure CLI
        STORAGE_KEY=$(az storage account keys list --account-name "$STORAGE_ACCOUNT" --query '[0].value' -o tsv 2>/dev/null || echo "")
        if [ -z "$STORAGE_KEY" ]; then
          echo "WARNING: Could not retrieve storage key; SMB mounts skipped." >&2
          exit 0
        fi
        mount -t cifs "//$${STORAGE_ACCOUNT}.file.core.windows.net/media"   /mnt/nas/media   -o "vers=3.0,username=$${STORAGE_ACCOUNT},password=$${STORAGE_KEY},serverino"
        mount -t cifs "//$${STORAGE_ACCOUNT}.file.core.windows.net/encodes" /mnt/nas/encodes -o "vers=3.0,username=$${STORAGE_ACCOUNT},password=$${STORAGE_KEY},serverino"
        mount -t cifs "//$${STORAGE_ACCOUNT}.file.core.windows.net/temp"    /mnt/nas/temp    -o "vers=3.0,username=$${STORAGE_ACCOUNT},password=$${STORAGE_KEY},serverino"
      fi
      echo "NAS mounts complete."

runcmd:
  # Enable and start Docker
  - systemctl enable docker
  - systemctl start docker
  # Add admin user to docker group
  - usermod -aG docker distencoder
  # Create config directories
  - mkdir -p /etc/distributed-encoder/certs
  # Fetch secrets from Key Vault (requires managed identity)
  - /usr/local/bin/fetch-kv-secrets.sh
  # Mount NAS shares
  - /usr/local/bin/mount-nas.sh
  # Start controller
  - cd /etc/distributed-encoder && docker compose up -d
  # Enable restart on reboot
  - systemctl enable docker
