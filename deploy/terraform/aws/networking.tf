# ── VPC ────────────────────────────────────────────────────────────────────────

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "distencoder-${var.environment}-vpc"
  }
}

# ── Subnets ────────────────────────────────────────────────────────────────────

# Public subnets — ALB, NAT gateways
resource "aws_subnet" "public" {
  count = 2

  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(var.vpc_cidr, 4, count.index)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "distencoder-${var.environment}-public-${count.index + 1}"
    Tier = "public"
  }
}

# Private subnets — controller, agents, RDS, EFS
resource "aws_subnet" "private" {
  count = 2

  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 4, count.index + 2)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "distencoder-${var.environment}-private-${count.index + 1}"
    Tier = "private"
  }
}

# ── Internet Gateway ───────────────────────────────────────────────────────────

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "distencoder-${var.environment}-igw"
  }
}

# ── NAT Gateway ────────────────────────────────────────────────────────────────

# Deploy a NAT gateway per AZ in HA mode so private subnets survive an AZ failure.
# In standard mode a single NAT gateway in the first public subnet is sufficient.
resource "aws_eip" "nat" {
  count  = var.enable_ha ? 2 : 1
  domain = "vpc"

  tags = {
    Name = "distencoder-${var.environment}-nat-eip-${count.index + 1}"
  }

  depends_on = [aws_internet_gateway.main]
}

resource "aws_nat_gateway" "main" {
  count = var.enable_ha ? 2 : 1

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  tags = {
    Name = "distencoder-${var.environment}-nat-${count.index + 1}"
  }

  depends_on = [aws_internet_gateway.main]
}

# ── Route Tables ───────────────────────────────────────────────────────────────

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "distencoder-${var.environment}-rt-public"
  }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Private subnets each get their own route table to point at the correct NAT
# gateway. In standard mode both private subnets share the single NAT gateway.
resource "aws_route_table" "private" {
  count  = 2
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = var.enable_ha ? aws_nat_gateway.main[count.index].id : aws_nat_gateway.main[0].id
  }

  tags = {
    Name = "distencoder-${var.environment}-rt-private-${count.index + 1}"
  }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

# ── RDS Subnet Group ───────────────────────────────────────────────────────────

resource "aws_db_subnet_group" "main" {
  name       = "distencoder-${var.environment}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "distencoder-${var.environment}-db-subnet-group"
  }
}
