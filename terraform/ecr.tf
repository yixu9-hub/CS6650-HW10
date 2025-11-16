# ===================================================
# 文件：terraform/ecr.tf
# 作用：创建该作业所需的全部 ECR repositories
# ===================================================

resource "aws_ecr_repository" "product" {
  name = "product-service"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "product_bad" {
  name = "product-service-bad"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "shopping_cart" {
  name = "shopping-cart-service"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "cca" {
  name = "credit-card-authorizer"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "warehouse" {
  name = "warehouse-consumer"
  image_scanning_configuration {
    scan_on_push = true
  }
}

# Optional：输出所有 repo 的 push/pull URL
output "ecr_urls" {
  description = "All ECR repo URLs"
  value = {
    product_service         = aws_ecr_repository.product.repository_url
    product_service_bad     = aws_ecr_repository.product_bad.repository_url
    shopping_cart_service   = aws_ecr_repository.shopping_cart.repository_url
    credit_card_authorizer  = aws_ecr_repository.cca.repository_url
    warehouse_consumer      = aws_ecr_repository.warehouse.repository_url
  }
}