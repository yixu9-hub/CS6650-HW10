# Terraform Infrastructure for E-commerce Microservices  
CS6650 – Homework 10

This folder contains the complete Terraform Infrastructure-as-Code (IaC) for deploying a fully serverless, Fargate-based e-commerce microservices system on AWS.  
All components run inside **AWS ECS Fargate**, load-balanced by an **Application Load Balancer (ALB)**, and communicate through **HTTP (internal over ALB)** and **AMQP (RabbitMQ)**.

---

# 1. Architecture Overview

```
         Local Load Testing Client
                     ↓ HTTP

┌──────────────────────────────────────────┐
│    AWS Application Load Balancer (ALB)   │
│            URL Path-Based Routing        │
└──────────────────────────────────────────┘
                     ↓
  /product*                 → Product Service Target Group
  /shopping-cart*           → Shopping Cart Service Target Group
  /authorize*               → Credit Card Authorizer (CCA) Target Group

Shopping Cart Service
  ↓ HTTP (via ALB)
Credit Card Authorizer (CCA)
  ↓ "Authorized" / "Declined"

RabbitMQ Queue (Fargate)
  ↓ AMQP
Warehouse Consumer (Fargate, multi-threaded)
```

All routing, health checks, service discovery, and isolation are handled by Terraform-provisioned AWS infrastructure.

---

# 2. Microservices Deployed

| Component | Description |
|----------|-------------|
| **Product Service** | Primary product API |
| **Bad Product Service** | 50% failure API (used to demonstrate ALB weighted routing) |
| **Shopping Cart Service** | Creates carts, adds items, performs checkout |
| **Credit Card Authorizer (CCA)** | Simulates payment authorization |
| **RabbitMQ** | Queue for checkout events |
| **Warehouse Consumer** | Reads events from RabbitMQ and processes orders |

All run as **Fargate tasks**, each with its own ECS service.

---

# 3. ALB Path-Based Routing

Terraform configures an Application Load Balancer with **three target groups**:

| Path Prefix | Route to Target Group |
|-------------|-------------------------|
| `/product*` | Product + Bad Product Services |
| `/shopping-cart*` | Shopping Cart Service |
| `/authorize*` | CCA Service |

This matches the behaviors used in your Go services.

### Health Checks (important)
| Service | Health Check Path | Valid Status |
|---------|-------------------|--------------|
| Product | `/products/1` | 200,404 |
| Shopping Cart | `/shopping-cart` | 405 (GET not allowed → service alive) |
| CCA | `/authorize` | 200–499 (GET returns 404/405 → still alive) |

These are **already correct** in your ALB Terraform.

---

# 4. What This Terraform Creates

✔ VPC networking integration (you provide VPC + subnets)  
✔ ALB + listeners + listener rules  
✔ ECR repositories  
✔ ECS Fargate task definitions  
✔ ECS services with public IP assignment  
✔ Weighted load balancing for good/bad product services  
✔ RabbitMQ + Warehouse consumer  
✔ Outputs for ALB DNS name & queue

---

# 5. Usage Instructions

## Step 1 — Copy example variables
```
cp terraform.tfvars.example terraform.tfvars
```

## Step 2 — Edit terraform.tfvars  
Fill in:

- your VPC ID  
- your subnet IDs  
- your ECR image URLs  
- rabbitmq_uri (after RabbitMQ task is up)  
- desired service counts  

## Step 3 — Terraform Init
```
terraform init
```

## Step 4 — Validate & Plan
```
terraform plan
```

## Step 5 — Apply infrastructure
```
terraform apply
```

After apply completes:

```
terraform output alb_dns_name
```

Save the returned DNS value:
```
http://ecommerce-alb-xxxx.us-west-2.elb.amazonaws.com
```

---

# 6. Build & Push All Docker Images

Each service must be built for **linux/amd64** (Fargate requirement):

Example (Product Service):
```
docker buildx build --platform linux/amd64 -t product-service .
docker tag product-service:latest 533267264147.dkr.ecr.us-west-2.amazonaws.com/product-service:latest
docker push 533267264147.dkr.ecr.us-west-2.amazonaws.com/product-service:latest
```

Repeat for:

- shopping-cart-service  
- credit-card-authorizer  
- product-service-bad  
- warehouse-consumer  

RabbitMQ uses Docker Hub → **no push needed**.

If you change code → rebuild → push → `terraform apply` again.

---

# 7. Testing the Deployment

Assume:
```
ALB=http://ecommerce-alb-xxxx.us-west-2.elb.amazonaws.com
```

### 1. Create a cart
```
curl -X POST "$ALB/shopping-cart" \
  -H "Content-Type: application/json" \
  -d '{"customer_id":1}'
```

### 2. Add item
```
curl -X POST "$ALB/shopping-carts/1/addItem" \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"quantity":2}'
```

### 3. Checkout (uses CCA)
```
curl -X POST "$ALB/shopping-carts/1/checkout" \
  -H "Content-Type: application/json" \
  -d '{"credit_card_number":"1234-5678-9012-3456"}'
```

### 4. Test CCA individually
```
curl -X POST "$ALB/authorize" \
  -H "Content-Type: application/json" \
  -d '{"credit_card_number":"1234-5678-9012-3456"}'
```

You should see:
```
Authorized
```
or
```
Declined
```

---

# 8. Load Testing Bad Product Instances (Required for HW10)

Because ALB uses **weighted_random** and you deployed:

- 2 good instances  
- 1 bad instance  

Running:

```
for i in {1..30}; do
  curl -s -o /dev/null -w "%{http_code}\n" "$ALB/product"
done
```

Expected distribution:  
- ~66% success (good service)  
- ~33% failure (bad service)  

---

# 9. Warehouse Queue Consumption

Warehouse consumer logs can be viewed in:
```
CloudWatch → Log groups → /ecs/warehouse-consumer
```

It will show:
- ACK of orders  
- Multi-threaded worker output  
- Messages received from RabbitMQ  

---

# 10. Clean Up

```
terraform destroy
```

---

# 11. Repository Notes

Items intentionally added to `.gitignore`:
- terraform.tfvars (real secrets)
- terraform.tfstate  
- terraform.tfstate.backup  
- .terraform/  
- docker-compose override  
- binaries / vendor  

`terraform.tfvars.example` **is** committed (required for grading).

---