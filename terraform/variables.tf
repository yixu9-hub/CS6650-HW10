# =========================
# 文件：terraform/variables.tf
# =========================

variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-west-2"
}

variable "vpc_id" {
  description = "VPC ID where resources will be created"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for ALB and ECS"
  type        = list(string)
}

variable "product_service_port" {
  description = "Port for product service"
  type        = number
  default     = 8080
}

variable "shopping_cart_service_port" {
  description = "Port for shopping cart service"
  type        = number
  default     = 8081
}

variable "cca_service_port" {
  description = "Port for credit card authorizer service"
  type        = number
  default     = 8082
}

# =========================
# 镜像地址（使用你的 ACCOUNT_ID）
# =========================

variable "product_service_image" {
  description = "ECR image for product service"
  type        = string
  default     = "533267264147.dkr.ecr.us-west-2.amazonaws.com/product-service:latest"
}

variable "product_service_bad_image" {
  description = "ECR image for bad product service"
  type        = string
  default     = "533267264147.dkr.ecr.us-west-2.amazonaws.com/product-service-bad:latest"
}

variable "shopping_cart_service_image" {
  description = "ECR image for shopping cart service"
  type        = string
  default     = "533267264147.dkr.ecr.us-west-2.amazonaws.com/shopping-cart-service:latest"
}

variable "cca_service_image" {
  description = "ECR image for credit card authorizer:latest"
  type        = string
  default     = "533267264147.dkr.ecr.us-west-2.amazonaws.com/credit-card-authorizer:latest"
}

# =========================
# NEW: RabbitMQ & Warehouse 镜像
# =========================

variable "rabbitmq_service_image" {
  description = "Docker image for RabbitMQ broker"
  type        = string
  # NOTE: 使用官方镜像即可，不一定要推到 ECR
  default     = "rabbitmq:3-management"
}

variable "warehouse_service_image" {
  description = "ECR image for warehouse consumer"
  type        = string
  default     = "533267264147.dkr.ecr.us-west-2.amazonaws.com/warehouse-consumer:latest"
}

# =========================
# ECS desired counts
# =========================

variable "product_service_desired_count" {
  description = "Desired count for product service (good instances)"
  type        = number
  default     = 2
}

variable "product_service_bad_desired_count" {
  description = "Desired count for bad product service"
  type        = number
  default     = 1
}

variable "shopping_cart_service_desired_count" {
  description = "Desired count for shopping cart service"
  type        = number
  default     = 1
}

variable "cca_service_desired_count" {
  description = "Desired count for credit card authorizer service"
  type        = number
  default     = 1
}

variable "rabbitmq_service_desired_count" {
  description = "Desired count for RabbitMQ broker service"
  type        = number
  default     = 1
}

variable "warehouse_service_desired_count" {
  description = "Desired count for warehouse consumer service"
  type        = number
  default     = 1
}

# =========================
# CHANGED: 删除了原来的 cca_internal_url 变量
# 之前这里有一个：
# variable "cca_internal_url" { default = "http://credit-card-authorizer:8082/authorize" }
# 这个 hostname 在 ECS 里解析不到，所以改为通过 ALB 访问，在 ecs.tf 中直接用 aws_lb.main.dns_name。
# =========================

# =========================
# RabbitMQ 连接 URI & Warehouse workers
# =========================

variable "rabbitmq_uri" {
  description = "AMQP URI for RabbitMQ broker (used by SCS & Warehouse)"
  type        = string
  # NOTE: 这里先用占位符，实际部署后建议在 terraform.tfvars 里覆盖成真实 IP：
  #       e.g. rabbitmq_uri = "amqp://guest:guest@3.91.23.45:5672/"
  default     = "amqp://guest:guest@RABBITMQ_HOST:5672/"
}

variable "warehouse_workers" {
  description = "Number of worker threads in the warehouse consumer"
  type        = number
  default     = 4
}