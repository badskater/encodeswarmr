#cloud-config
# Agent cloud-init — installs .deb package, encoding tools, mounts Azure Files,
# pulls mTLS certs from Key Vault, then starts encodeswarmr-agent service.

package_update: true
package_upgrade: true

packages:
  - curl
  - jq
  - ffmpeg
  - nfs-common
  - cifs-utils
  - azure-cli
  - libx265-dev
  - libx264-dev

write_files:
  - path: /etc/encodeswarmr-agent/agent.yaml
    permissions: "0640"
    owner: root:encodeswarmr
    content: |
      controller:
        address: "${controller_address}"
        tls:
          cert: "/etc/encodeswarmr-agent/certs/agent.crt"
          key:  "/etc/encodeswarmr-agent/certs/agent.key"
          ca:   "/etc/encodeswarmr-agent/certs/ca.crt"
        reconnect:
          initial_delay: 5s
          max_delay: 5m
          multiplier: 2.0

      agent:
        hostname: ""
        work_dir: "/var/lib/encodeswarmr-agent/work"
        log_dir:  "/var/log/encodeswarmr-agent"
        offline_db: "/var/lib/encodeswarmr-agent/offline.db"
        heartbeat_interval: 30s
        poll_interval: 10s
        cleanup_on_success: true
        keep_failed_jobs: 10

      tools:
        ffmpeg:  "/usr/bin/ffmpeg"
        ffprobe: "/usr/bin/ffprobe"
        x265:    "/usr/bin/x265"
        x264:    "/usr/bin/x264"
        svt_av1: ""
        avs_pipe: ""
        vspipe:  ""

      gpu:
        enabled: false
        vendor: ""
        max_vram_mb: 0
        monitor_interval: 5s

      allowed_shares:
        - "/mnt/nas/media"
        - "/mnt/nas/encodes"

      logging:
        level: info
        format: json
        max_size_mb: 100
        max_backups: 5
        compress: true
        stream_buffer_size: 1000
        stream_flush_interval: 1s

  - path: /usr/local/bin/fetch-agent-secrets.sh
    permissions: "0750"
    owner: root:root
    content: |
      #!/bin/bash
      # Fetch agent mTLS certs from Azure Key Vault using the VM managed identity.
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

      mkdir -p /etc/encodeswarmr-agent/certs
      chmod 750 /etc/encodeswarmr-agent/certs

      get_secret "agent-cert" > /etc/encodeswarmr-agent/certs/agent.crt
      get_secret "agent-key"  > /etc/encodeswarmr-agent/certs/agent.key
      get_secret "ca-cert"    > /etc/encodeswarmr-agent/certs/ca.crt
      chmod 640 /etc/encodeswarmr-agent/certs/*.crt /etc/encodeswarmr-agent/certs/*.key

      echo "Agent secrets fetched successfully."

  - path: /usr/local/bin/mount-nas.sh
    permissions: "0750"
    owner: root:root
    content: |
      #!/bin/bash
      set -euo pipefail

      STORAGE_ACCOUNT="${storage_account}"
      NFS_ENABLED="${nfs_enabled}"

      mkdir -p /mnt/nas/media /mnt/nas/encodes /mnt/nas/temp

      if [ "$NFS_ENABLED" = "true" ]; then
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/media"   /mnt/nas/media   -o vers=4,minorversion=1,sec=sys
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/encodes" /mnt/nas/encodes -o vers=4,minorversion=1,sec=sys
        mount -t nfs "$${STORAGE_ACCOUNT}.file.core.windows.net:/$${STORAGE_ACCOUNT}/temp"    /mnt/nas/temp    -o vers=4,minorversion=1,sec=sys
      else
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
  # Create service user
  - useradd -r -m -s /bin/bash encodeswarmr || true
  # Install encodeswarmr-agent .deb
  - >
    curl -fsSL
    "https://github.com/badskater/encodeswarmr/releases/download/v${encodeswarmr_version}/encodeswarmr-agent_${encodeswarmr_version}_linux_amd64.deb"
    -o /tmp/agent.deb
  - dpkg -i /tmp/agent.deb || apt-get install -f -y
  - rm /tmp/agent.deb
  # Install SVT-AV1 encoder
  - apt-get install -y svt-av1 || true
  # Fetch mTLS certs from Key Vault
  - /usr/local/bin/fetch-agent-secrets.sh
  # Mount NAS shares
  - /usr/local/bin/mount-nas.sh
  # Set correct permissions
  - chown -R encodeswarmr:encodeswarmr /var/lib/encodeswarmr-agent /var/log/encodeswarmr-agent
  - chown -R root:encodeswarmr /etc/encodeswarmr-agent
  # Enable and start agent service
  - systemctl daemon-reload
  - systemctl enable encodeswarmr-agent
  - systemctl start encodeswarmr-agent
