variable "app_name" { type = string }
variable "environment" { type = string }
variable "vpc_id" { type = string }
variable "private_subnet_ids" { type = list(string) }
variable "ecs_tasks_sg_id" { type = string }
variable "api_tg_arn" { type = string }
variable "task_execution_role_arn" { type = string }
variable "task_role_arn" { type = string }
variable "aws_region" { type = string }
variable "log_group_name" { type = string }

# ── Service images ────────────────────────────────────────────────────────────
variable "ingestor_image" { type = string }
variable "classifier_image" { type = string }
variable "api_image" { type = string }

# ── Sizing ────────────────────────────────────────────────────────────────────
variable "ingestor_cpu" { type = number }
variable "ingestor_memory" { type = number }
variable "classifier_cpu" { type = number }
variable "classifier_memory" { type = number }
variable "api_cpu" { type = number }
variable "api_memory" { type = number }
variable "api_desired_count" { type = number }

# ── Secrets ARNs (injected from secrets.tf outputs) ──────────────────────────
variable "secrets_arn" { type = string }

resource "aws_ecs_cluster" "main" {
  name = "${var.app_name}-${var.environment}"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_ecs_cluster_capacity_providers" "fargate" {
  cluster_name       = aws_ecs_cluster.main.name
  capacity_providers = ["FARGATE", "FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
}

# ── Ingestor (scheduled — runs on a cron trigger, not a long-lived service) ──

resource "aws_ecs_task_definition" "ingestor" {
  family                   = "${var.app_name}-ingestor"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.ingestor_cpu
  memory                   = var.ingestor_memory
  execution_role_arn       = var.task_execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "ingestor"
    image     = var.ingestor_image
    essential = true
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "ingestor"
      }
    }
    secrets = [
      { name = "REDDIT_RAPIDAPI_KEY", valueFrom = "${var.secrets_arn}:REDDIT_RAPIDAPI_KEY::" },
      { name = "MONGO_URI", valueFrom = "${var.secrets_arn}:MONGO_URI::" },
      { name = "REDIS_URL", valueFrom = "${var.secrets_arn}:REDIS_URL::" },
    ]
    environment = [
      { name = "REDDIT_RAPIDAPI_HOST", value = "reddit34.p.rapidapi.com" },
    ]
  }])
}

# ── Classifier ────────────────────────────────────────────────────────────────

resource "aws_ecs_task_definition" "classifier" {
  family                   = "${var.app_name}-classifier"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.classifier_cpu
  memory                   = var.classifier_memory
  execution_role_arn       = var.task_execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name         = "classifier"
    image        = var.classifier_image
    essential    = true
    portMappings = [{ containerPort = 8000, protocol = "tcp" }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "classifier"
      }
    }
    secrets = [
      { name = "MONGO_URI", valueFrom = "${var.secrets_arn}:MONGO_URI::" },
      { name = "REDIS_URL", valueFrom = "${var.secrets_arn}:REDIS_URL::" },
      { name = "QDRANT_URL", valueFrom = "${var.secrets_arn}:QDRANT_URL::" },
      { name = "QDRANT_API_KEY", valueFrom = "${var.secrets_arn}:QDRANT_API_KEY::" },
      { name = "ANTHROPIC_API_KEY", valueFrom = "${var.secrets_arn}:ANTHROPIC_API_KEY::" },
      { name = "DATABASE_URL", valueFrom = "${var.secrets_arn}:DATABASE_URL::" },
      { name = "PRODUCTHUNT_API_TOKEN", valueFrom = "${var.secrets_arn}:PRODUCTHUNT_API_TOKEN::" },
    ]
    environment = [
      { name = "LLM_MODEL_DEV", value = "claude-haiku-4-5-20251001" },
      { name = "EMBEDDING_MODEL", value = "nomic-embed-text" },
      { name = "CLASSIFIER_WORKERS", value = "3" },
    ]
  }])
}

resource "aws_ecs_service" "classifier" {
  name            = "${var.app_name}-classifier"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.classifier.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.ecs_tasks_sg_id]
    assign_public_ip = false
  }

  lifecycle {
    ignore_changes = [desired_count]
  }
}

# ── API ───────────────────────────────────────────────────────────────────────

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.app_name}-api"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = var.task_execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name         = "api"
    image        = var.api_image
    essential    = true
    portMappings = [{ containerPort = 4000, protocol = "tcp" }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "api"
      }
    }
    secrets = [
      { name = "MONGO_URI", valueFrom = "${var.secrets_arn}:MONGO_URI::" },
      { name = "DATABASE_URL", valueFrom = "${var.secrets_arn}:DATABASE_URL::" },
      { name = "REDIS_URL", valueFrom = "${var.secrets_arn}:REDIS_URL::" },
      { name = "AUTH0_DOMAIN", valueFrom = "${var.secrets_arn}:AUTH0_DOMAIN::" },
      { name = "AUTH0_AUDIENCE", valueFrom = "${var.secrets_arn}:AUTH0_AUDIENCE::" },
      { name = "STRIPE_SECRET_KEY", valueFrom = "${var.secrets_arn}:STRIPE_SECRET_KEY::" },
      { name = "STRIPE_WEBHOOK_SECRET", valueFrom = "${var.secrets_arn}:STRIPE_WEBHOOK_SECRET::" },
      { name = "RESEND_API_KEY", valueFrom = "${var.secrets_arn}:RESEND_API_KEY::" },
    ]
    environment = [
      { name = "NODE_ENV", value = "production" },
      { name = "PORT", value = "4000" },
    ]
  }])
}

resource "aws_ecs_service" "api" {
  name            = "${var.app_name}-api"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.ecs_tasks_sg_id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = var.api_tg_arn
    container_name   = "api"
    container_port   = 4000
  }

  lifecycle {
    ignore_changes = [desired_count, task_definition]
  }
}

output "cluster_name" { value = aws_ecs_cluster.main.name }
output "cluster_arn" { value = aws_ecs_cluster.main.arn }
output "ingestor_task_def_arn" { value = aws_ecs_task_definition.ingestor.arn }
