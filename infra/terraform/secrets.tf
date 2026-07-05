resource "aws_secretsmanager_secret" "app" {
  name                    = "${var.app_name}/${var.environment}/app"
  description             = "WantedBy application secrets"
  recovery_window_in_days = var.environment == "prod" ? 30 : 0
}

locals {
  # Build the secrets map after RDS and ElastiCache are created.
  secret_values = {
    MONGO_URI             = var.mongo_uri
    DATABASE_URL          = "postgresql://${var.db_username}:${random_password.db.result}@${module.rds.endpoint}/${var.db_name}"
    REDIS_URL             = "rediss://${module.elasticache.primary_endpoint}:${module.elasticache.port}"
    AUTH0_DOMAIN          = var.auth0_domain
    AUTH0_AUDIENCE        = var.auth0_audience
    STRIPE_SECRET_KEY     = var.stripe_secret_key
    STRIPE_WEBHOOK_SECRET = var.stripe_webhook_secret
    RESEND_API_KEY        = var.resend_api_key
    REDDIT_RAPIDAPI_KEY   = var.reddit_rapidapi_key
    ANTHROPIC_API_KEY     = var.anthropic_api_key
    PRODUCTHUNT_API_TOKEN = var.producthunt_api_token
    QDRANT_URL            = "https://qdrant.${var.domain_name}"
    QDRANT_API_KEY        = var.qdrant_api_key
  }
}

resource "aws_secretsmanager_secret_version" "app" {
  secret_id     = aws_secretsmanager_secret.app.id
  secret_string = jsonencode(local.secret_values)

  lifecycle {
    # Prevent accidental rotation destroying secret content.
    ignore_changes = [secret_string]
  }
}

resource "random_password" "db" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}
