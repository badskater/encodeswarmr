# High Availability Setup

This document describes the active-passive HA model for the distributed-encoder controller.

## Overview

Two (or more) controller replicas run simultaneously. Only one — the **leader** — actively processes jobs and serves agent gRPC connections. The others are **standby** nodes. If the leader crashes or loses its database connection, a standby acquires the leader lock within one heartbeat interval (≤ 5 s) and takes over.

Both replicas connect to the same PostgreSQL instance (or Patroni cluster) and the same NFS/NAS share. No state is held exclusively in memory on the leader.

## Leader Election

Leader election uses a **PostgreSQL session-level advisory lock** (`pg_try_advisory_lock`). The lock is held for the lifetime of the database connection:

- Each controller polls `pg_try_advisory_lock` every 5 s.
- The first instance to acquire the lock logs `"ha: became leader"` and begins processing.
- All other instances remain idle, polling until they can acquire the lock.
- If the lock holder's connection drops (crash, SIGKILL, network partition), PostgreSQL automatically releases the lock and a standby acquires it on its next heartbeat.
- On clean shutdown, `pg_advisory_unlock` is called explicitly so promotion happens within 5 s rather than waiting for a TCP timeout.

## Node Identity

Each controller identifies itself with a `node_id` derived from `os.Hostname()`. Override this with the `HA_NODE_ID` environment variable when multiple replicas share the same hostname (e.g. Docker Swarm tasks, Kubernetes pods).

## Endpoints

| Endpoint | Auth | Description |
|---|---|---|
| `GET /health` | None | Returns `{"status":"ok","leader":true/false,"db":"ok"}`. Use this as a load-balancer health check. |
| `GET /api/v1/ha/status` | None | Returns `{"leader":true/false,"node_id":"..."}`. |

## Agent Reconnection

Agents require no HA-specific configuration. The existing exponential-backoff retry loop in `register()` handles controller failover transparently:

1. The agent's gRPC connection gets `codes.Unavailable` (or a network error) when the primary fails.
2. The agent logs the error and backs off using the configured `reconnect.*` delays (default: 5 s initial, 2× multiplier, 5 min cap).
3. The load balancer routes the next retry to the newly promoted leader.
4. Any task results or log lines that could not be delivered during the failover window are buffered in the local SQLite offline journal and replayed after reconnection.

No sticky sessions are required on the load balancer. Agents reconnect to whichever controller instance is currently the leader.

## Docker Compose Example

The snippet below shows two controller replicas behind an nginx reverse proxy. Both replicas connect to the same PostgreSQL and NFS mount. The `HA_NODE_ID` variable distinguishes the nodes in logs.

```yaml
version: "3.9"

services:
  # --- PostgreSQL (single node; use Patroni for production HA) ---
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: encoder
      POSTGRES_USER: encoder
      POSTGRES_PASSWORD: changeme
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U encoder"]
      interval: 5s
      retries: 10

  # --- Controller replica 1 (may become leader) ---
  controller-1:
    image: ghcr.io/badskater/distributed-encoder/controller:latest
    environment:
      HA_NODE_ID: controller-1
    volumes:
      - ./config.yaml:/etc/distributed-encoder/config.yaml:ro
      - /mnt/nas:/mnt/nas
    depends_on:
      postgres:
        condition: service_healthy

  # --- Controller replica 2 (hot standby) ---
  controller-2:
    image: ghcr.io/badskater/distributed-encoder/controller:latest
    environment:
      HA_NODE_ID: controller-2
    volumes:
      - ./config.yaml:/etc/distributed-encoder/config.yaml:ro
      - /mnt/nas:/mnt/nas
    depends_on:
      postgres:
        condition: service_healthy

  # --- Load balancer (sticky sessions NOT required) ---
  nginx:
    image: nginx:alpine
    ports:
      - "8080:8080"   # HTTP API
      - "9443:9443"   # gRPC
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - controller-1
      - controller-2

volumes:
  pgdata:
```

### Minimal nginx.conf

```nginx
events {}

http {
    upstream controller_http {
        server controller-1:8080;
        server controller-2:8080;
    }
    server {
        listen 8080;
        location / {
            proxy_pass http://controller_http;
        }
    }
}

stream {
    upstream controller_grpc {
        server controller-1:9443;
        server controller-2:9443;
    }
    server {
        listen 9443;
        proxy_pass controller_grpc;
    }
}
```

## PostgreSQL HA with Patroni

For full database HA, replace the single `postgres` container with a Patroni cluster. Key considerations:

- Configure Patroni with a DCS backend (etcd, Consul, or ZooKeeper).
- Point the controller `database.url` at the Patroni VIP or a HAProxy sidecar that routes writes to the primary.
- The controller advisory lock is held on the primary; if Patroni promotes a replica, the old leader's connection drops and a new election occurs automatically.
- Use `synchronous_standby_names` in PostgreSQL to prevent data loss on failover.

See the [Patroni documentation](https://patroni.readthedocs.io/) for cluster setup details.

## Failure Scenarios

| Scenario | Recovery time | Notes |
|---|---|---|
| Leader process crash | ≤ 5 s | Advisory lock released by PostgreSQL; standby promotes on next heartbeat. |
| Leader graceful shutdown | < 1 s | `pg_advisory_unlock` called before exit. |
| Network partition (leader loses DB) | ≤ 5 s | DB query timeout triggers lock-loss; standby promotes. |
| PostgreSQL primary failover (Patroni) | Patroni failover time + ≤ 5 s | Controller reconnects to new primary; advisory lock re-acquired. |
| All controllers down | Until one restarts | Agents buffer results offline; jobs resume when a controller comes back. |
