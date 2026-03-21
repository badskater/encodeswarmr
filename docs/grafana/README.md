# Grafana Dashboard — Distributed Encoder

This directory contains a pre-built Grafana dashboard for monitoring the
distributed-encoder controller and its encoding agents.

## Dashboard file

| File | Description |
|------|-------------|
| `distributed-encoder.json` | Main operational dashboard (schema version 37, Grafana 10+) |

## Panels

| # | Panel | Query |
|---|-------|-------|
| 1 | Job Throughput | `increase(distencoder_jobs_total{status="completed"}[1h])` |
| 2 | Task Duration Histogram (p50 / p95 / p99) | `histogram_quantile(0.N, rate(distencoder_task_duration_seconds_bucket[5m]))` |
| 3 | Agent CPU / GPU Utilization | `avg by (agent_id) (agent_cpu_pct)` / `agent_gpu_pct` |
| 4 | Queue Depth | `distencoder_tasks_total{status="pending"}` |
| 5 | Error Rate | failed / (failed + completed) ratio for tasks and jobs |
| 6 | Active Agents | `distencoder_active_agents` gauge |
| 7 | Encoding FPS | `distencoder_encoding_fps` across all running tasks |
| 8 | HTTP Request Rate | `rate(distencoder_http_requests_total[1m])` by endpoint |

## Dashboard variables

| Variable | Type | Purpose |
|----------|------|---------|
| `$agent` | Query | Filter agent CPU/GPU panels by agent ID |
| `$interval` | Interval | Step interval for rate/increase functions |

## Import instructions

### Grafana UI (recommended)

1. Open your Grafana instance and navigate to **Dashboards → Import**.
2. Click **Upload dashboard JSON file** and select
   `docs/grafana/distributed-encoder.json`.
3. On the import screen, select your Prometheus datasource from the
   **Prometheus** dropdown.
4. Click **Import**.

### Grafana provisioning (automated)

Copy the JSON file into your Grafana provisioning dashboards directory and add
a provisioning configuration file if you do not already have one:

```yaml
# grafana/provisioning/dashboards/default.yaml
apiVersion: 1
providers:
  - name: distributed-encoder
    type: file
    updateIntervalSeconds: 60
    options:
      path: /etc/grafana/dashboards
```

Then place `distributed-encoder.json` at `/etc/grafana/dashboards/` (or
whichever path your provisioning config points to) and restart Grafana (or
wait for the reload interval).

### Grafana API

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -u admin:password \
  http://localhost:3000/api/dashboards/import \
  -d @docs/grafana/distributed-encoder.json
```

## Prometheus scrape configuration

Add the controller `/metrics` endpoint to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: distributed-encoder
    static_configs:
      - targets:
          - controller:8080   # adjust host:port as needed
    metrics_path: /metrics
    scrape_interval: 15s
```

The `/metrics` endpoint is unauthenticated and returns data in Prometheus text
exposition format 0.0.4.

## Exposed metrics

| Metric | Type | Description |
|--------|------|-------------|
| `distencoder_jobs_total` | Gauge | Job count by status |
| `distencoder_tasks_total` | Gauge | Task count by status and task_type |
| `distencoder_agents_total` | Gauge | Agent count by status |
| `distencoder_active_agents` | Gauge | Agents in running or idle state |
| `distencoder_encoding_fps` | Gauge | Current total encoding FPS |
| `distencoder_task_duration_seconds` | Histogram | Task execution time (started → completed) |
| `distencoder_task_queue_wait_seconds` | Histogram | Time tasks wait in pending state |
| `distencoder_chunk_throughput_bytes` | Counter | Total encoded output bytes |
| `distencoder_grpc_requests_total` | Counter | gRPC call count by method |
| `distencoder_http_requests_total` | Counter | HTTP request count by method, path, and status |
