# ── IAM Role for Agent Instances ───────────────────────────────────────────────

resource "aws_iam_role" "agent" {
  name = "distencoder-${var.environment}-agent-role"

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
    Name = "distencoder-${var.environment}-agent-role"
  }
}

resource "aws_iam_role_policy" "agent_ssm" {
  name = "distencoder-${var.environment}-agent-ssm-policy"
  role = aws_iam_role.agent.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # Agents only need to read their cert and the CA — not controller key or secrets
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters",
        ]
        Resource = [
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/distencoder/${var.environment}/certs/ca.crt",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/distencoder/${var.environment}/certs/agent.crt",
          "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/distencoder/${var.environment}/certs/agent.key",
        ]
      },
      {
        # SSM Session Manager — shell access without opening port 22
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

resource "aws_iam_instance_profile" "agent" {
  name = "distencoder-${var.environment}-agent-profile"
  role = aws_iam_role.agent.name
}

# ── Controller gRPC endpoint for agent config ──────────────────────────────────
# In HA mode agents connect via the NLB DNS name; in standard mode they connect
# directly to the first controller instance's private DNS.

locals {
  # NLB DNS is only available in HA mode; otherwise use the ASG instance DNS
  # The agent config references this at boot — resolved via local at plan time.
  controller_grpc_endpoint = var.enable_ha ? "${aws_lb.grpc[0].dns_name}:${var.controller_grpc_port}" : "controller.distencoder.internal:${var.controller_grpc_port}"
}

# ── Agent User Data ────────────────────────────────────────────────────────────

locals {
  agent_userdata = <<-USERDATA
    #!/bin/bash
    set -euo pipefail
    exec > >(tee /var/log/distencoder-agent-init.log | logger -t distencoder-agent-init) 2>&1

    echo "=== distributed-encoder agent bootstrap ==="
    echo "Version: ${var.distencoder_version}"
    echo "Environment: ${var.environment}"

    # ── System update and encoding toolchain ──────────────────────────────────
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -q
    apt-get install -yq \
      nfs-common \
      awscli \
      jq \
      curl \
      ffmpeg \
      x264 \
      x265 \
      libsvtav1enc0 \
      svt-av1

    # ── Mount EFS shared storage ──────────────────────────────────────────────
    EFS_DNS="${aws_efs_file_system.main.dns_name}"

    for dir in /mnt/nas/media /mnt/nas/encodes /mnt/nas/temp; do
      mkdir -p "$dir"
    done

    mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport \
      "$EFS_DNS":/ /mnt/nas/media || true

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
      --name "/distencoder/${var.environment}/certs/agent.crt" \
      --query "Parameter.Value" --output text > "$CERT_DIR/agent.crt"

    aws ssm get-parameter \
      --region "${var.region}" \
      --name "/distencoder/${var.environment}/certs/agent.key" \
      --with-decryption \
      --query "Parameter.Value" --output text > "$CERT_DIR/agent.key"

    chmod 600 "$CERT_DIR/agent.key" "$CERT_DIR/ca.crt" "$CERT_DIR/agent.crt"

    # ── Download and install agent .deb ───────────────────────────────────────
    RELEASE_URL="https://github.com/badskater/distributed-encoder/releases/download/v${var.distencoder_version}/distributed-encoder-agent_${var.distencoder_version}_amd64.deb"
    curl -fsSL "$RELEASE_URL" -o /tmp/distencoder-agent.deb
    dpkg -i /tmp/distencoder-agent.deb || apt-get install -f -y
    rm -f /tmp/distencoder-agent.deb

    # ── Write agent config ────────────────────────────────────────────────────
    HOSTNAME=$(hostname -f)

    cat > /etc/distributed-encoder/agent.yaml <<EOF
controller:
  address: "${local.controller_grpc_endpoint}"
  tls:
    cert: "/etc/distributed-encoder/certs/agent.crt"
    key:  "/etc/distributed-encoder/certs/agent.key"
    ca:   "/etc/distributed-encoder/certs/ca.crt"
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  hostname: "$HOSTNAME"
  work_dir: "/var/lib/distributed-encoder-agent/work"
  log_dir:  "/var/log/distributed-encoder-agent"
  offline_db: "/var/lib/distributed-encoder-agent/offline.db"
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
  vspipe:   ""

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

vnc:
  enabled: false
  port: 5900
EOF

    # ── Enable and start agent service ────────────────────────────────────────
    systemctl daemon-reload
    systemctl enable distributed-encoder-agent
    systemctl start distributed-encoder-agent

    echo "=== Agent bootstrap complete ==="
  USERDATA
}

# ── Launch Template ────────────────────────────────────────────────────────────

resource "aws_launch_template" "agent" {
  name_prefix   = "distencoder-${var.environment}-agent-"
  image_id      = data.aws_ami.ubuntu_24_04.id
  instance_type = var.agent_instance_type

  iam_instance_profile {
    name = aws_iam_instance_profile.agent.name
  }

  network_interfaces {
    associate_public_ip_address = false
    security_groups             = [aws_security_group.agent.id]
    delete_on_termination       = true
  }

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required" # IMDSv2
    http_put_response_hop_limit = 1
  }

  block_device_mappings {
    device_name = "/dev/sda1"
    ebs {
      # Larger root for encode working space on the instance itself
      volume_size           = 200
      volume_type           = "gp3"
      encrypted             = true
      delete_on_termination = true
    }
  }

  user_data = base64encode(local.agent_userdata)

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name = "distencoder-${var.environment}-agent"
      Role = "agent"
    }
  }

  tags = {
    Name = "distencoder-${var.environment}-agent-lt"
  }

  lifecycle {
    create_before_destroy = true
  }
}

# ── Auto Scaling Group ─────────────────────────────────────────────────────────

resource "aws_autoscaling_group" "agent" {
  name                = "distencoder-${var.environment}-agent-asg"
  vpc_zone_identifier = aws_subnet.private[*].id

  min_size         = var.agent_count
  max_size         = var.agent_count * 4
  desired_capacity = var.agent_count

  health_check_type         = "EC2"
  health_check_grace_period = 300

  launch_template {
    id      = aws_launch_template.agent.id
    version = "$Latest"
  }

  instance_refresh {
    strategy = "Rolling"
    preferences {
      min_healthy_percentage = 50
    }
  }

  tag {
    key                 = "Name"
    value               = "distencoder-${var.environment}-agent"
    propagate_at_launch = true
  }

  tag {
    key                 = "Role"
    value               = "agent"
    propagate_at_launch = true
  }

  depends_on = [
    aws_autoscaling_group.controller,
    aws_efs_mount_target.main,
    aws_ssm_parameter.agent_cert,
    aws_ssm_parameter.agent_key,
    aws_ssm_parameter.ca_cert,
  ]
}
