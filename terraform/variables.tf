variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "vpc_id" {
  description = "VPC ID where resources will be created"
  type        = string
}

variable "subnet_ids" {
  description = "List of subnet IDs for ALB"
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
