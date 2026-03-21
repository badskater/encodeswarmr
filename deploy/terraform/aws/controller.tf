# ── IAM Role for Controller Instances ─────────────────────────────────────────

resource "aws_iam_role" "controller" {
  name = "distencoder-${var.environment}-controller-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "distencoder-${var.environment}-controller-role"
  }
}

resource "aws_iam_role_policy" "controller_ssm" {
  name = "distencoder-${var.environment}-controller-ssm-policy"
  role = aws_iam_role.controller.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # Allow the controller to read its certs and secrets from SSM
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath",
        ]
        Resource = "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/distencoder/${var.environment}/*"
      },
      {
        # SSM Session Manager — allows shell access without opening SSH port
        Effect = "Allow"
        Action = [
          "ssmmessages:CreateControlChannel",
          "ssmmessages:CreateDataChannel",
          "ssmmessages:OpenControlChannel",
          "ssmmessages:OpenDataChannel",
          "ssm:UpdateInstanceInformation",
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_instance_profile" "controller" {
  name = "distencoder-${var.environment}-controller-profile"
  role = aws_iam_role.controller.name
}

# ── Controller User Data ───────────────────────────────────────────────────────

locals {
  controller_userdata = <<-USERDATA
    #!/bin/bash
    set -euo pipefail
    exec > >(tee /var/log/distencoder-init.log | logger -t distencoder-init) 2>&1

    echo "=== distributed-encoder controller bootstrap ==="
    echo "Version: ${var.distencoder_version}"
    echo "Environment: ${var.environment}"

    # ── System update and base packages ──────────────────────────────────────
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -q
    apt-get install -yq \
      docker.io \
      nfs-common \
      awscli \
      jq \
      ffmpeg \
      curl

    systemctl enable docker
    systemctl start docker

    # ── Mount EFS shared storage ──────────────────────────────────────────────
    EFS_DNS="${aws_efs_file_system.main.dns_name}"

    for dir in /mnt/nas/media /mnt/nas/encodes /mnt/nas/temp; do
      mkdir -p "$dir"
    done

    mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport \
      "$EFS_DNS":/ /mnt/nas/media || true

    # Persist EFS mounts across reboots
    cat >> /etc/fstab <<EOF
$EFS_DNS:/ /mnt/nas/media    nfs4 nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport,_netdev 0 0
EOF

    # ── Fetch mTLS certificates from SSM ──────────────────────────────────────
    CERT_DIR=/etc/distributed-encoder/certs
    mkdir -p "$CERT_DIR"
    chmod 700 "$CERT_DIR"

    aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/certs/ca.crt" \
      --query "Parameter.Value" --output text > "$CERT_DIR/ca.crt"

    aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/certs/controller.crt" \
      --query "Parameter.Value" --output text > "$CERT_DIR/controller.crt"

    aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/certs/controller.key" \
      --with-decryption \
      --query "Parameter.Value" --output text > "$CERT_DIR/controller.key"

    chmod 600 "$CERT_DIR/controller.key" "$CERT_DIR/ca.crt" "$CERT_DIR/controller.crt"

    # ── Fetch application secrets ─────────────────────────────────────────────
    DB_PASSWORD=$(aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/db/password" \
      --with-decryption \
      --query "Parameter.Value" --output text)

    SESSION_SECRET=$(aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/controller/session_secret" \
      --with-decryption \
      --query "Parameter.Value" --output text)

    # ── Write controller config ───────────────────────────────────────────────
    CONFIG_DIR=/etc/distributed-encoder
    mkdir -p "$CONFIG_DIR"

    cat > "$CONFIG_DIR/controller.yaml" <<EOF
server:
  host: "0.0.0.0"
  port: ${var.controller_http_port}
  read_timeout: 30s
  write_timeout: 30s

database:
  url: "postgres://${var.db_username}:$DB_PASSWORD@${aws_db_instance.main.address}:5432/${var.db_name}"
  max_conns: 25
  min_conns: 5
  max_conn_lifetime: 1h
  migrations_path: "/usr/share/distributed-encoder/migrations"

grpc:
  host: "0.0.0.0"
  port: ${var.controller_grpc_port}
  tls:
    cert: "/etc/distributed-encoder/certs/controller.crt"
    key:  "/etc/distributed-encoder/certs/controller.key"
    ca:   "/etc/distributed-encoder/certs/ca.crt"

auth:
  session_ttl: 24h
  session_secret: "$SESSION_SECRET"
  oidc:
    enabled: ${var.enable_oidc}

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

webhooks:
  worker_count: 4
  delivery_timeout: 10s
  max_retries: 3

tls:
  cert: "/etc/distributed-encoder/certs/controller.crt"
  key:  "/etc/distributed-encoder/certs/controller.key"
  ca:   "/etc/distributed-encoder/certs/ca.crt"

analysis:
  ffmpeg_bin:  "/usr/bin/ffmpeg"
  ffprobe_bin: "/usr/bin/ffprobe"
  concurrency: 2

  path_mappings:
    - name:    "NAS media"
      windows: "\\\\NAS01\\media"
      linux:   "/mnt/nas/media"
    - name:    "NAS encodes"
      windows: "\\\\NAS01\\encodes"
      linux:   "/mnt/nas/encodes"
    - name:    "NAS temp"
      windows: "\\\\NAS01\\temp"
      linux:   "/mnt/nas/temp"
EOF

    # ── Pull and start controller container ───────────────────────────────────
    docker pull "ghcr.io/badskater/distributed-encoder:${var.distencoder_version}" || \
      docker pull "ghcr.io/badskater/distributed-encoder:latest"

    docker run -d \
      --name distencoder-controller \
      --restart unless-stopped \
      -p ${var.controller_http_port}:${var.controller_http_port} \
      -p ${var.controller_grpc_port}:${var.controller_grpc_port} \
      -v /etc/distributed-encoder:/etc/distributed-encoder:ro \
      -v /mnt/nas:/mnt/nas \
      "ghcr.io/badskater/distributed-encoder:${var.distencoder_version}" \
      controller --config /etc/distributed-encoder/controller.yaml

    echo "=== Controller bootstrap complete ==="
  USERDATA
}

# ── Launch Template ────────────────────────────────────────────────────────────

resource "aws_launch_template" "controller" {
  name_prefix   = "distencoder-${var.environment}-controller-"
  image_id      = data.aws_ami.ubuntu_24_04.id
  instance_type = var.controller_instance_type

  iam_instance_profile {
    name = aws_iam_instance_profile.controller.name
  }

  network_interfaces {
    associate_public_ip_address = false
    security_groups             = [aws_security_group.controller.id]
    delete_on_termination       = true
  }

  dynamic "key_name" {
    for_each = var.ssh_key_name != "" ? [var.ssh_key_name] : []
    content {}
  }

  # Workaround: key_name in launch template doesn't support dynamic blocks;
  # use metadata_options to set key_name via local
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required" # IMDSv2 required
    http_put_response_hop_limit = 1
  }

  block_device_mappings {
    device_name = "/dev/sda1"
    ebs {
      volume_size           = 30
      volume_type           = "gp3"
      encrypted             = true
      delete_on_termination = true
    }
  }

  user_data = base64encode(local.controller_userdata)

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name = "distencoder-${var.environment}-controller"
      Role = "controller"
    }
  }

  tags = {
    Name = "distencoder-${var.environment}-controller-lt"
  }

  lifecycle {
    create_before_destroy = true
  }
}

# ── Auto Scaling Group ─────────────────────────────────────────────────────────

resource "aws_autoscaling_group" "controller" {
  name                = "distencoder-${var.environment}-controller-asg"
  vpc_zone_identifier = aws_subnet.private[*].id

  min_size         = var.enable_ha ? 2 : 1
  max_size         = var.enable_ha ? 2 : 1
  desired_capacity = var.enable_ha ? 2 : 1

  health_check_type         = "EC2"
  health_check_grace_period = 300

  launch_template {
    id      = aws_launch_template.controller.id
    version = "$Latest"
  }

  # Attach to NLB target group in HA mode (defined in loadbalancer.tf)
  target_group_arns = var.enable_ha ? [aws_lb_target_group.grpc[0].arn, aws_lb_target_group.http[0].arn] : []

  instance_refresh {
    strategy = "Rolling"
    preferences {
      min_healthy_percentage = 50
    }
  }

  tag {
    key                 = "Name"
    value               = "distencoder-${var.environment}-controller"
    propagate_at_launch = true
  }

  tag {
    key                 = "Role"
    value               = "controller"
    propagate_at_launch = true
  }

  depends_on = [
    aws_db_instance.main,
    aws_efs_mount_target.main,
    aws_ssm_parameter.controller_cert,
    aws_ssm_parameter.controller_key,
    aws_ssm_parameter.ca_cert,
    aws_ssm_parameter.db_password,
    aws_ssm_parameter.session_secret,
  ]
}
