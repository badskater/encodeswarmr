# Ansible Deployment — EncodeSwarmr

Ansible playbooks for deploying `encodeswarmr` to on-premise systems.

Three deployment modes are provided:

| Mode | Playbook | Description |
|------|----------|-------------|
| Standard | `playbooks/site.yml` | Single controller + agents |
| HA | `playbooks/ha.yml` | Two controllers + HAProxy + optional Patroni |
| Docker Compose | `docker-compose/deploy.yml` | Full stack on a single Docker host |

---

## Prerequisites

- Ansible 2.15+
- Python 3.9+ on the control node
- Required Python packages:

```bash
pip install ansible pywinrm psycopg2-binary
```

- Required Ansible collections:

```bash
ansible-galaxy collection install ansible.windows ansible.posix community.postgresql
```

- `openssl` available on the Ansible control node (for mTLS cert generation)
- Linux target hosts: SSH access with sudo
- Windows target hosts: WinRM enabled (see [WinRM setup](#windows-agent-winrm-setup))

---

## Quick start (standard deployment)

```bash
cd deploy/ansible

# 1. Create your inventory
cp inventory/hosts.yml.example inventory/hosts.yml
# Edit inventory/hosts.yml with your IP addresses and credentials

# 2. Create Ansible Vault for secrets
ansible-vault create inventory/group_vars/vault.yml
# Add inside:
#   vault_db_password: "your-db-password"
#   vault_session_secret: "your-session-secret"
#   vault_oidc_client_secret: ""   # optional

# 3. Review variables
# Edit inventory/group_vars/all.yml      — shared settings
# Edit inventory/group_vars/controllers.yml  — controller settings
# Edit inventory/group_vars/agents.yml    — agent settings

# 4. Deploy everything
ansible-playbook -i inventory/hosts.yml playbooks/site.yml --ask-vault-pass
```

---

## Available playbooks

### `playbooks/site.yml` — Full deployment

Deploys database, controller, all Linux agents, and all Windows agents in order.

```bash
ansible-playbook -i inventory/hosts.yml playbooks/site.yml --ask-vault-pass
```

### `playbooks/controller.yml` — Controller only

Useful when adding a new controller to an existing installation.

```bash
ansible-playbook -i inventory/hosts.yml playbooks/controller.yml --ask-vault-pass
```

### `playbooks/agents.yml` — Agents only

Deploy or update all agents. Use `--limit` to target a specific group.

```bash
# All agents
ansible-playbook -i inventory/hosts.yml playbooks/agents.yml --ask-vault-pass

# Linux agents only
ansible-playbook -i inventory/hosts.yml playbooks/agents.yml \
  --limit agents_linux --ask-vault-pass

# Windows agents only
ansible-playbook -i inventory/hosts.yml playbooks/agents.yml \
  --limit agents_windows --ask-vault-pass
```

### `playbooks/ha.yml` — High-Availability deployment

Deploys two controllers, HAProxy load balancer, and optionally a Patroni PostgreSQL cluster.

```bash
# With standalone PostgreSQL
ansible-playbook -i inventory/hosts.yml playbooks/ha.yml --ask-vault-pass

# With Patroni HA PostgreSQL
ansible-playbook -i inventory/hosts.yml playbooks/ha.yml \
  -e use_patroni=true --ask-vault-pass
```

Required inventory groups: `controllers_ha`, `loadbalancer`, and either `database` or `patroni`.

### `playbooks/upgrade.yml` — Rolling upgrade

Upgrades all components one host at a time. Set the target version in `group_vars/all.yml`
(`encodeswarmr_version`) or pass it on the command line.

```bash
# Upgrade to version set in group_vars
ansible-playbook -i inventory/hosts.yml playbooks/upgrade.yml --ask-vault-pass

# Override version at runtime
ansible-playbook -i inventory/hosts.yml playbooks/upgrade.yml \
  -e encodeswarmr_version=1.1.0 --ask-vault-pass

# Upgrade only Linux agents
ansible-playbook -i inventory/hosts.yml playbooks/upgrade.yml \
  --tags agent-linux --ask-vault-pass
```

### `docker-compose/deploy.yml` — Docker Compose alternative

Installs Docker on the target host and runs the full stack via Docker Compose.

```bash
ansible-playbook -i inventory/hosts.yml docker-compose/deploy.yml --ask-vault-pass
```

---

## Variable reference

### group_vars/all.yml (shared)

| Variable | Default | Description |
|----------|---------|-------------|
| `encodeswarmr_version` | `"1.0.4"` | Release version to deploy |
| `encodeswarmr_github_repo` | `"badskater/encodeswarmr"` | GitHub repo |
| `controller_http_port` | `8080` | Controller REST API / web UI port |
| `controller_grpc_port` | `9443` | Controller gRPC + mTLS port |
| `db_host` | `"localhost"` | PostgreSQL host |
| `db_port` | `5432` | PostgreSQL port |
| `db_name` | `"encodeswarmr"` | Database name |
| `db_user` | `"encodeswarmr"` | Database user |
| `db_password` | `"{{ vault_db_password }}"` | Database password (from Vault) |
| `cert_dir` | `"/etc/encodeswarmr/certs"` | mTLS cert directory (Linux) |
| `mtls_generate_certs` | `true` | Generate certs automatically |
| `mtls_ca_days` | `3650` | CA cert validity (days) |
| `mtls_cert_days` | `1825` | Leaf cert validity (days) |
| `nas_nfs_server` | `"nas01.local"` | NFS server hostname |
| `nas_nfs_exports` | see all.yml | NFS mounts for Linux hosts |
| `nas_smb_shares` | see all.yml | UNC shares for Windows agents |
| `controller_log_level` | `"info"` | Controller log level |
| `agent_heartbeat_interval` | `"30s"` | Agent heartbeat interval |
| `agent_poll_interval` | `"10s"` | Agent job poll interval |

### group_vars/controllers.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `controller_session_secret` | `"{{ vault_session_secret }}"` | Session signing key |
| `controller_oidc_enabled` | `false` | Enable OIDC auth |
| `controller_auto_approve` | `false` | Auto-approve new agents |
| `controller_analysis_concurrency` | `2` | Controller-side analysis concurrency |
| `controller_path_mappings` | see controllers.yml | NAS path mappings seeded on first run |

### group_vars/agents.yml

| Variable | Default | Description |
|----------|---------|-------------|
| `agent_controller_address` | `"controller-01:9443"` | Controller gRPC address |
| `windows_install_dir` | `C:\DistEncoder` | Windows agent install root |
| `windows_ffmpeg_bin` | `C:\Tools\ffmpeg\ffmpeg.exe` | FFmpeg path on Windows |
| `linux_ffmpeg_bin` | `/usr/bin/ffmpeg` | FFmpeg path on Linux |
| `agent_gpu_enabled` | `true` | Enable GPU monitoring |

---

## Windows agent WinRM setup

WinRM must be enabled on each Windows agent before Ansible can connect.

Run the following in an elevated PowerShell on each Windows host:

```powershell
# Enable WinRM with HTTPS (recommended)
winrm quickconfig -quiet
Enable-PSRemoting -Force

# Create self-signed cert for WinRM HTTPS
$cert = New-SelfSignedCertificate -DnsName $env:COMPUTERNAME -CertStoreLocation Cert:\LocalMachine\My
New-WSManInstance -ResourceURI winrm/config/Listener `
  -SelectorSet @{Address="*"; Transport="HTTPS"} `
  -ValueSet @{Hostname=$env:COMPUTERNAME; CertificateThumbprint=$cert.Thumbprint}

# Allow WinRM through the firewall
netsh advfirewall firewall add rule name="WinRM HTTPS" dir=in action=allow protocol=TCP localport=5986

# Verify
winrm enumerate winrm/config/Listener
```

Inventory must include:
```yaml
ansible_connection: winrm
ansible_winrm_transport: ntlm
ansible_port: 5986
ansible_winrm_server_cert_validation: ignore
```

---

## mTLS certificates

By default (`mtls_generate_certs: true`), the `mtls-certs` role generates a CA,
controller cert, and agent cert on the Ansible control node and stores them in
`roles/mtls-certs/files/`. These are then copied to each host.

To use pre-provisioned certificates, set `mtls_generate_certs: false` and place
the following files in `roles/mtls-certs/files/` before running any playbook:

```
roles/mtls-certs/files/
  ca.crt
  ca.key
  controller.crt
  controller.key
  agent.crt
  agent.key
```

**Keep `ca.key` and `*.key` files secure.** They are only needed on the control node
and should not be committed to version control.

To regenerate certificates (e.g. on expiry), delete the files in `roles/mtls-certs/files/`
and re-run the playbook with `--tags certs`.

---

## Docker Compose alternative

Use `docker-compose/deploy.yml` when you want to run the controller in Docker rather
than installing the .deb package directly. This is useful for quick evaluations or
when the target host already runs Docker.

```bash
# Prerequisites: add docker_hosts group to inventory/hosts.yml
ansible-playbook -i inventory/hosts.yml docker-compose/deploy.yml --ask-vault-pass
```

The playbook:
1. Installs Docker Engine + Compose plugin
2. Creates `/opt/encodeswarmr/` with templated config
3. Copies mTLS certs into `/opt/encodeswarmr/certs/`
4. Writes secrets to `/opt/encodeswarmr/.env`
5. Runs `docker compose up -d`

NFS shares must be pre-mounted on the Docker host (or mount them via the `common` role).

---

## Upgrading

Use `playbooks/upgrade.yml` for rolling upgrades with no downtime:

```bash
ansible-playbook -i inventory/hosts.yml playbooks/upgrade.yml \
  -e encodeswarmr_version=1.1.0 --ask-vault-pass
```

The playbook upgrades each component (`serial: 1`) and waits for health checks
before proceeding to the next host.

---

## Tags reference

All tasks are tagged. Use `--tags` to run only specific phases:

| Tag | Description |
|-----|-------------|
| `certs` | mTLS cert generation and distribution |
| `install` | Package download and installation |
| `configure` | Config file templating |
| `service` | Service enable / start / restart |
| `nfs` | NFS mount setup |
| `common` | Common system setup (NTP, packages) |
| `postgresql` | PostgreSQL install and configuration |
| `controller` | Controller-specific tasks |
| `agent` | All agent tasks |
| `agent-linux` | Linux agent tasks only |
| `agent-windows` | Windows agent tasks only |
| `haproxy` | HAProxy install and configuration |
| `patroni` | Patroni HA PostgreSQL tasks |

---

## Troubleshooting

### Certificate errors

**Symptom:** Agent cannot connect to controller; TLS handshake fails.

- Verify cert files exist on the agent host:
  - Linux: `ls -la /etc/encodeswarmr/certs/`
  - Windows: `dir C:\DistEncoder\certs\`
- Verify cert and CA match: `openssl verify -CAfile ca.crt agent.crt`
- Re-distribute certs: `ansible-playbook playbooks/site.yml --tags certs`

### Controller service not starting

```bash
# Check service status and logs
sudo systemctl status encodeswarmr-controller
sudo journalctl -u encodeswarmr-controller -n 50 --no-pager
```

Common causes: database not reachable, missing cert files, config YAML syntax error.

### Agent service not starting (Linux)

```bash
sudo systemctl status encodeswarmr-agent
sudo journalctl -u encodeswarmr-agent -n 50 --no-pager
```

### Agent service not starting (Windows)

```powershell
Get-Service encodeswarmr-agent
Get-EventLog -LogName Application -Source 'encodeswarmr-agent' -Newest 20
```

### WinRM connectivity

```bash
# Test from control node
ansible agents_windows -i inventory/hosts.yml -m ansible.windows.win_ping
```

If this fails, check that WinRM HTTPS listener is running on port 5986 and the
firewall allows inbound connections on that port.

### PostgreSQL connection refused

- Confirm `db_host` in `group_vars/all.yml` is correct
- Check `pg_hba.conf` allows connections from the controller host
- Run the postgresql role again: `ansible-playbook playbooks/site.yml --tags postgresql`

### NFS mount failures

- Verify NFS server is reachable: `showmount -e nas01.local`
- Check `nfs-common` is installed: `dpkg -l nfs-common`
- Inspect mount errors: `sudo dmesg | grep nfs`
