variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment (prod | staging)"
  type        = string
  default     = "prod"

  validation {
    condition     = contains(["prod", "staging"], var.environment)
    error_message = "environment must be prod or staging"
  }
}

variable "app_name" {
  description = "Application name used as a prefix for resource names"
  type        = string
  default     = "wantedby"
}

# ── Networking ────────────────────────────────────────────────────────────────

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones to deploy into (at least 2 for RDS Multi-AZ)"
  type        = list(string)
  default     = ["us-east-1a", "us-east-1b"]
}

# ── ECS ───────────────────────────────────────────────────────────────────────

variable "ingestor_image" {
  description = "Full ECR image URI for the ingestor service"
  type        = string
}

variable "classifier_image" {
  description = "Full ECR image URI for the classifier service"
  type        = string
}

variable "api_image" {
  description = "Full ECR image URI for the API service"
  type        = string
}

variable "ingestor_cpu" {
  description = "CPU units for ingestor task (256 = 0.25 vCPU)"
  type        = number
  default     = 256
}

variable "ingestor_memory" {
  description = "Memory (MiB) for ingestor task"
  type        = number
  default     = 512
}

variable "classifier_cpu" {
  type    = number
  default = 1024
}

variable "classifier_memory" {
  type    = number
  default = 2048
}

variable "api_cpu" {
  type    = number
  default = 512
}

variable "api_memory" {
  type    = number
  default = 1024
}

variable "api_desired_count" {
  description = "Number of running API task replicas"
  type        = number
  default     = 2
}

# ── RDS ───────────────────────────────────────────────────────────────────────

variable "rds_instance_class" {
  description = "RDS instance type"
  type        = string
  default     = "db.t3.micro"
}

variable "rds_allocated_storage" {
  description = "RDS allocated storage (GiB)"
  type        = number
  default     = 20
}

variable "db_name" {
  type    = string
  default = "wantedby_db"
}

variable "db_username" {
  type    = string
  default = "wantedby"
}

# ── ElastiCache ───────────────────────────────────────────────────────────────

variable "redis_node_type" {
  type    = string
  default = "cache.t3.micro"
}

# ── Domain ────────────────────────────────────────────────────────────────────

variable "domain_name" {
  description = "Root domain (e.g. wantedby.io) — must exist as a Route 53 hosted zone"
  type        = string
  default     = "wantedby.io"
}

variable "acm_certificate_arn" {
  description = "ACM certificate ARN for the domain (must be in us-east-1 for CloudFront)"
  type        = string
  default     = ""
}

# ── Secrets (passed from CI/CD, never in source) ──────────────────────────────

variable "auth0_domain" {
  type      = string
  sensitive = true
}

variable "auth0_audience" {
  type      = string
  sensitive = true
}

variable "stripe_secret_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "stripe_webhook_secret" {
  type      = string
  sensitive = true
  default   = ""
}

variable "resend_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "reddit_rapidapi_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "anthropic_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "producthunt_api_token" {
  type      = string
  sensitive = true
  default   = ""
}

variable "mongo_uri" {
  description = "MongoDB Atlas connection string"
  type        = string
  sensitive   = true
}

variable "qdrant_api_key" {
  type      = string
  sensitive = true
  default   = ""
}
