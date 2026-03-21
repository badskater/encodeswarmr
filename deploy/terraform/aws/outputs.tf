# ── Controller ─────────────────────────────────────────────────────────────────

output "controller_url" {
  description = "HTTP URL for the controller web UI and REST API."
  value = var.enable_ha ? (
    "http://${aws_lb.http[0].dns_name}:${var.controller_http_port}"
    ) : (
    "Connect via SSM Session Manager — no public HTTP endpoint in standard mode. Use an SSH tunnel or deploy an ALB."
  )
}

output "grpc_endpoint" {
  description = "gRPC endpoint for agent configuration (controller_address in agent.yaml)."
  value = var.enable_ha ? (
    "${aws_lb.grpc[0].dns_name}:${var.controller_grpc_port}"
    ) : (
    "Use the private DNS of the controller EC2 instance with port ${var.controller_grpc_port}."
  )
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer (HA mode only)."
  value       = var.enable_ha ? aws_lb.http[0].dns_name : "N/A — ALB is not deployed in standard mode."
}

output "nlb_dns_name" {
  description = "DNS name of the Network Load Balancer for gRPC (HA mode only)."
  value       = var.enable_ha ? aws_lb.grpc[0].dns_name : "N/A — NLB is not deployed in standard mode."
}

# ── Database ───────────────────────────────────────────────────────────────────

output "rds_endpoint" {
  description = "RDS PostgreSQL connection endpoint (host:port)."
  value       = "${aws_db_instance.main.address}:${aws_db_instance.main.port}"
}

output "rds_db_name" {
  description = "Name of the PostgreSQL database."
  value       = aws_db_instance.main.db_name
}

# ── Storage ────────────────────────────────────────────────────────────────────

output "efs_id" {
  description = "EFS file system ID. Used to mount shared storage on additional instances."
  value       = aws_efs_file_system.main.id
}

output "efs_dns_name" {
  description = "EFS DNS name for NFS mounts."
  value       = aws_efs_file_system.main.dns_name
}

# ── Auto Scaling Groups ────────────────────────────────────────────────────────

output "controller_asg_name" {
  description = "Name of the controller Auto Scaling Group."
  value       = aws_autoscaling_group.controller.name
}

output "agent_asg_name" {
  description = "Name of the agent Auto Scaling Group."
  value       = aws_autoscaling_group.agent.name
}

# ── Networking ─────────────────────────────────────────────────────────────────

output "vpc_id" {
  description = "VPC ID."
  value       = aws_vpc.main.id
}

output "private_subnet_ids" {
  description = "IDs of the private subnets."
  value       = aws_subnet.private[*].id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets."
  value       = aws_subnet.public[*].id
}

# ── SSH / Session Manager ──────────────────────────────────────────────────────

output "ssh_commands" {
  description = "Example commands for connecting to instances via AWS SSM Session Manager (no open SSH port required)."
  value       = <<-EOT
    # List controller instances:
    aws ec2 describe-instances \
      --filters "Name=tag:Role,Values=controller" "Name=tag:Environment,Values=${var.environment}" \
                "Name=instance-state-name,Values=running" \
      --query "Reservations[*].Instances[*].[InstanceId,PrivateIpAddress,Tags[?Key=='Name']|[0].Value]" \
      --output table

    # Connect to a controller via SSM (replace INSTANCE_ID):
    aws ssm start-session --target INSTANCE_ID

    # List agent instances:
    aws ec2 describe-instances \
      --filters "Name=tag:Role,Values=agent" "Name=tag:Environment,Values=${var.environment}" \
                "Name=instance-state-name,Values=running" \
      --query "Reservations[*].Instances[*].[InstanceId,PrivateIpAddress,Tags[?Key=='Name']|[0].Value]" \
      --output table
  EOT
}

# ── SSM Paths ──────────────────────────────────────────────────────────────────

output "ssm_parameter_prefix" {
  description = "SSM Parameter Store path prefix for all encodeswarmr secrets."
  value       = "/encodeswarmr/${var.environment}/"
}
