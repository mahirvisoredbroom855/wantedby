# GitHub Actions Workflows

## Workflows

| File | Trigger | Purpose |
|------|---------|---------|
| `ci.yml` | PR to `develop`/`main`, push to `develop` | Lint, typecheck, test all 4 services |
| `deploy.yml` | Push to `main` (services/) | Build + ECR push + ECS rolling deploy |
| `frontend-deploy.yml` | Push to `main` (frontend/) | Build + S3 sync + CloudFront invalidation |

## Required GitHub Secrets

Set these under **Settings → Secrets and variables → Actions**:

### AWS credentials
| Secret | Value |
|--------|-------|
| `AWS_ACCESS_KEY_ID` | IAM user access key (deploy permissions only) |
| `AWS_SECRET_ACCESS_KEY` | IAM user secret key |

### Frontend build
| Secret | Value |
|--------|-------|
| `VITE_API_URL` | `https://your-domain.com` |
| `VITE_AUTH0_DOMAIN` | Auth0 domain (e.g. `dev-xxx.us.auth0.com`) |
| `VITE_AUTH0_CLIENT_ID` | Auth0 SPA client ID |
| `VITE_AUTH0_AUDIENCE` | Auth0 API audience |

### Deploy targets
| Secret | Value |
|--------|-------|
| `FRONTEND_BUCKET` | S3 bucket name (Terraform output: `frontend_bucket_name`) |
| `CLOUDFRONT_DISTRIBUTION_ID` | CloudFront distribution ID (Terraform output: `distribution_id`) |

## GitHub Environment

`deploy.yml` and `frontend-deploy.yml` use a `production` environment — create it under **Settings → Environments** to add required reviewers or deployment protection rules.

## IAM policy for CI/CD

The AWS IAM user needs:
- `ecr:GetAuthorizationToken`, `ecr:BatchCheckLayerAvailability`, `ecr:PutImage`, etc. (ECR push)
- `ecs:UpdateService`, `ecs:DescribeServices` (rolling deploys)
- `s3:PutObject`, `s3:DeleteObject`, `s3:ListBucket` (frontend sync)
- `cloudfront:CreateInvalidation` (cache bust)
