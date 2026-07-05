variable "app_name" { type = string }
variable "environment" { type = string }
variable "subnet_ids" { type = list(string) }
variable "security_group_id" { type = string }
variable "instance_class" { type = string }
variable "allocated_storage" { type = number }
variable "db_name" { type = string }
variable "db_username" { type = string }
variable "db_password" {
  type      = string
  sensitive = true
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.app_name}-${var.environment}-db-subnet"
  subnet_ids = var.subnet_ids
}

resource "aws_db_instance" "postgres" {
  identifier                = "${var.app_name}-${var.environment}-postgres"
  engine                    = "postgres"
  engine_version            = "16.3"
  instance_class            = var.instance_class
  allocated_storage         = var.allocated_storage
  storage_type              = "gp3"
  storage_encrypted         = true
  db_name                   = var.db_name
  username                  = var.db_username
  password                  = var.db_password
  db_subnet_group_name      = aws_db_subnet_group.main.name
  vpc_security_group_ids    = [var.security_group_id]
  multi_az                  = var.environment == "prod"
  backup_retention_period   = 7
  deletion_protection       = var.environment == "prod"
  skip_final_snapshot       = var.environment != "prod"
  final_snapshot_identifier = var.environment == "prod" ? "${var.app_name}-final-snapshot" : null

  tags = { Name = "${var.app_name}-postgres" }
}

output "endpoint" { value = aws_db_instance.postgres.endpoint }
output "db_name" { value = aws_db_instance.postgres.db_name }
