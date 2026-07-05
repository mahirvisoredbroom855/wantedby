locals {
  name_prefix = "${var.app_name}-${var.environment}"
}

# ── CloudWatch log group (shared by all ECS tasks) ────────────────────────────

resource "aws_cloudwatch_log_group" "app" {
  name              = "/ecs/${local.name_prefix}"
  retention_in_days = 30
}

# ── VPC ───────────────────────────────────────────────────────────────────────

module "vpc" {
  source             = "./modules/vpc"
  app_name           = var.app_name
  environment        = var.environment
  vpc_cidr           = var.vpc_cidr
  availability_zones = var.availability_zones
}

# ── ECR ───────────────────────────────────────────────────────────────────────

module "ecr" {
  source      = "./modules/ecr"
  app_name    = var.app_name
  environment = var.environment
}

# ── RDS (PostgreSQL) ──────────────────────────────────────────────────────────

module "rds" {
  source            = "./modules/rds"
  app_name          = var.app_name
  environment       = var.environment
  subnet_ids        = module.vpc.private_subnet_ids
  security_group_id = module.vpc.rds_sg_id
  instance_class    = var.rds_instance_class
  allocated_storage = var.rds_allocated_storage
  db_name           = var.db_name
  db_username       = var.db_username
  db_password       = random_password.db.result
}

# ── ElastiCache (Redis) ───────────────────────────────────────────────────────

module "elasticache" {
  source            = "./modules/elasticache"
  app_name          = var.app_name
  environment       = var.environment
  subnet_ids        = module.vpc.private_subnet_ids
  security_group_id = module.vpc.redis_sg_id
  node_type         = var.redis_node_type
}

# ── ALB ───────────────────────────────────────────────────────────────────────

module "alb" {
  source              = "./modules/alb"
  app_name            = var.app_name
  environment         = var.environment
  vpc_id              = module.vpc.vpc_id
  public_subnet_ids   = module.vpc.public_subnet_ids
  security_group_id   = module.vpc.alb_sg_id
  acm_certificate_arn = var.acm_certificate_arn
}

# ── ECS ───────────────────────────────────────────────────────────────────────

module "ecs" {
  source                  = "./modules/ecs"
  app_name                = var.app_name
  environment             = var.environment
  vpc_id                  = module.vpc.vpc_id
  private_subnet_ids      = module.vpc.private_subnet_ids
  ecs_tasks_sg_id         = module.vpc.ecs_tasks_sg_id
  api_tg_arn              = module.alb.api_tg_arn
  task_execution_role_arn = aws_iam_role.task_execution.arn
  task_role_arn           = aws_iam_role.task.arn
  aws_region              = var.aws_region
  log_group_name          = aws_cloudwatch_log_group.app.name
  secrets_arn             = aws_secretsmanager_secret.app.arn
  ingestor_image          = var.ingestor_image
  classifier_image        = var.classifier_image
  api_image               = var.api_image
  ingestor_cpu            = var.ingestor_cpu
  ingestor_memory         = var.ingestor_memory
  classifier_cpu          = var.classifier_cpu
  classifier_memory       = var.classifier_memory
  api_cpu                 = var.api_cpu
  api_memory              = var.api_memory
  api_desired_count       = var.api_desired_count
}

# ── CloudFront + S3 (frontend) ────────────────────────────────────────────────

module "cloudfront" {
  source              = "./modules/cloudfront"
  app_name            = var.app_name
  environment         = var.environment
  domain_name         = var.domain_name
  acm_certificate_arn = var.acm_certificate_arn
  api_alb_dns_name    = module.alb.alb_dns_name
}

# ── EventBridge rule: run ingestor every 2 hours ──────────────────────────────
# (Reddit quota is 50/month so HN runs on schedule, Reddit only on --hn-once)

data "aws_iam_policy_document" "events_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "eventbridge_ecs" {
  name               = "${local.name_prefix}-eventbridge-ecs"
  assume_role_policy = data.aws_iam_policy_document.events_assume_role.json
}

resource "aws_iam_role_policy" "eventbridge_ecs" {
  role = aws_iam_role.eventbridge_ecs.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["ecs:RunTask"]
      Resource = [module.ecs.ingestor_task_def_arn]
      }, {
      Effect   = "Allow"
      Action   = ["iam:PassRole"]
      Resource = [aws_iam_role.task_execution.arn, aws_iam_role.task.arn]
    }]
  })
}

resource "aws_cloudwatch_event_rule" "ingestor" {
  name                = "${local.name_prefix}-ingestor-schedule"
  description         = "Run ingestor every 2 hours for HN; Reddit only via --reddit-once flag"
  schedule_expression = "rate(2 hours)"
}

resource "aws_cloudwatch_event_target" "ingestor" {
  rule     = aws_cloudwatch_event_rule.ingestor.name
  arn      = module.ecs.cluster_arn
  role_arn = aws_iam_role.eventbridge_ecs.arn

  ecs_target {
    task_definition_arn = module.ecs.ingestor_task_def_arn
    task_count          = 1
    launch_type         = "FARGATE"

    network_configuration {
      subnets          = module.vpc.private_subnet_ids
      security_groups  = [module.vpc.ecs_tasks_sg_id]
      assign_public_ip = false
    }

    # Pass --hn-once flag; Reddit fetched separately via manual trigger to preserve quota.
    container_overrides {
      container_override {
        name    = "ingestor"
        command = ["/ingestor", "--hn-once"]
      }
    }
  }
}
