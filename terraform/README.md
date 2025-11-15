# Terraform ALB Configuration for E-commerce Microservices

This Terraform configuration creates an Application Load Balancer (ALB) with target groups for three microservices:
- Product Service
- Shopping Cart Service
- Credit Card Authorizer Service

## Features

- **Path-based routing**: Routes requests based on URL patterns
  - `/product*` → Product Service
  - `/shopping-cart*` → Shopping Cart Service
  - `/credit-card-authorizer*` → Credit Card Authorizer Service

- **Automatic target weights**: Uses `weighted_random` algorithm to manage bad instances
- **Health checks**: Configured for each service
- **Security groups**: Allows HTTP/HTTPS traffic

## Prerequisites

1. AWS account with appropriate permissions
2. VPC with at least 2 subnets in different availability zones
3. EC2 instances running the microservices

## Usage

1. Copy the example variables file:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. Edit `terraform.tfvars` with your AWS configuration:
   - VPC ID
   - Subnet IDs (at least 2 in different AZs)
   - Service ports (if different from defaults)

3. Initialize Terraform:
   ```bash
   terraform init
   ```

4. Review the plan:
   ```bash
   terraform plan
   ```

5. Apply the configuration:
   ```bash
   terraform apply
   ```

6. Get the ALB DNS name:
   ```bash
   terraform output alb_dns_name
   ```

## Registering Targets

After creating the ALB, register your EC2 instances with the target groups:

```bash
# Get target group ARNs
terraform output product_target_group_arn
terraform output shopping_cart_target_group_arn
terraform output cca_target_group_arn

# Register instances (replace with your instance IDs and target group ARNs)
aws elbv2 register-targets \
  --target-group-arn <TARGET_GROUP_ARN> \
  --targets Id=<INSTANCE_ID>
```

## Testing

Test the routing:

```bash
# Product service
curl http://<ALB_DNS_NAME>/products/123

# Shopping cart service
curl -X POST http://<ALB_DNS_NAME>/shopping-cart \
  -H "Content-Type: application/json" \
  -d '{"customer_id": 1}'

# Credit card authorizer
curl -X POST http://<ALB_DNS_NAME>/credit-card-authorizer/authorize \
  -H "Content-Type: application/json" \
  -d '{"credit_card_number": "1234-5678-9012-3456"}'
```

## Cleanup

To destroy all resources:

```bash
terraform destroy
```
