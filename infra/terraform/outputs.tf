output "cloudfront_url" {
  description = "Frontend URL (use this in VITE_API_URL after domain setup)"
  value       = "https://${module.cloudfront.cloudfront_domain}"
}

output "api_url" {
  description = "Direct ALB URL for the GraphQL API"
  value       = "http://${module.alb.alb_dns_name}"
}

output "ecr_repository_urls" {
  description = "ECR image URLs for CI/CD push"
  value       = module.ecr.repository_urls
}

output "ecs_cluster_name" {
  value = module.ecs.cluster_name
}

output "frontend_bucket_name" {
  description = "S3 bucket to deploy frontend build into"
  value       = module.cloudfront.frontend_bucket_name
}

output "cloudfront_distribution_id" {
  description = "CloudFront distribution ID — needed for cache invalidation after deploy"
  value       = module.cloudfront.distribution_id
}

output "rds_endpoint" {
  description = "RDS endpoint (without port)"
  value       = module.rds.endpoint
  sensitive   = true
}
