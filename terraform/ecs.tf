# =========================================
# 文件：terraform/ecs.tf
# ECS Cluster + HTTP 微服务（Product / Bad / SCS / CCA）
# =========================================

# ECS Cluster
resource "aws_ecs_cluster" "main" {
  name = "ecommerce-cluster"
}

# 引用 IAM role "labrole"
data "aws_iam_role" "labrole" {
  name = "labrole"
}

# Security Group for ECS Services
resource "aws_security_group" "ecs_service" {
  name        = "ecommerce-ecs-sg"
  description = "Security group for ECS Fargate services"
  vpc_id      = var.vpc_id

  ingress {
    from_port       = 0
    to_port         = 65535
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
    description     = "Allow traffic from ALB"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "ecommerce-ecs-sg"
  }
}

# =========================================
# Product Service Task Definition
# =========================================
resource "aws_ecs_task_definition" "product" {
  family                   = "product-service-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "product-service"
      image = var.product_service_image
      portMappings = [
        {
          containerPort = var.product_service_port
          hostPort      = var.product_service_port
          protocol      = "tcp"
        }
      ]
      essential = true
      environment = [
        {
          name  = "SERVICE_NAME"
          value = "product-service"
        }
      ]
    }
  ])
}

# =========================================
# Bad Product Service Task Definition
# =========================================
resource "aws_ecs_task_definition" "product_bad" {
  family                   = "product-service-bad-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "product-service-bad"
      image = var.product_service_bad_image
      portMappings = [
        {
          containerPort = var.product_service_port
          hostPort      = var.product_service_port
          protocol      = "tcp"
        }
      ]
      essential = true
      environment = [
        {
          name  = "SERVICE_NAME"
          value = "product-service-bad"
        }
      ]
    }
  ])
}

# =========================================
# Shopping Cart Service Task Definition
# =========================================
resource "aws_ecs_task_definition" "shopping_cart" {
  family                   = "shopping-cart-service-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "shopping-cart-service"
      image = var.shopping_cart_service_image
      portMappings = [
        {
          containerPort = var.shopping_cart_service_port
          hostPort      = var.shopping_cart_service_port
          protocol      = "tcp"
        }
      ]
      essential = true
      environment = [
        # ★ CHANGED: 这里只是 ALB 的基础 URL，不带任何 path
        #     SCS 代码里会拼：CCA_URL + "/credit-card-authorizer/authorize"
        {
          name  = "CCA_URL"
          value = "http://${aws_lb.main.dns_name}"
        },
        # RABBITMQ_URI：SCS 用它向队列发布消息
        {
          name  = "RABBITMQ_URI"
          value = var.rabbitmq_uri
        }
      ]
    }
  ])
}

# =========================================
# Credit Card Authorizer Task Definition
# =========================================
resource "aws_ecs_task_definition" "cca" {
  family                   = "cca-service-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "credit-card-authorizer"
      image = var.cca_service_image
      portMappings = [
        {
          containerPort = var.cca_service_port
          hostPort      = var.cca_service_port
          protocol      = "tcp"
        }
      ]
      essential = true
    }
  ])
}

# =========================================
# ECS Service - Product Service (good instances)
# =========================================
resource "aws_ecs_service" "product" {
  name            = "product-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.product.arn
  desired_count   = var.product_service_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.product.arn
    container_name   = "product-service"
    container_port   = var.product_service_port
  }

  depends_on = [
    aws_lb_listener.http
  ]
}

# =========================================
# ECS Service - Bad Product Service
# =========================================
resource "aws_ecs_service" "product_bad" {
  name            = "product-service-bad"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.product_bad.arn
  desired_count   = var.product_service_bad_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.product.arn
    container_name   = "product-service-bad"
    container_port   = var.product_service_port
  }

  depends_on = [
    aws_lb_listener.http
  ]
}

# =========================================
# ECS Service - Shopping Cart Service
# =========================================
resource "aws_ecs_service" "shopping_cart" {
  name            = "shopping-cart-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.shopping_cart.arn
  desired_count   = var.shopping_cart_service_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.shopping_cart.arn
    container_name   = "shopping-cart-service"
    container_port   = var.shopping_cart_service_port
  }

  depends_on = [
    aws_lb_listener.http
  ]
}

# =========================================
# ECS Service - Credit Card Authorizer
# =========================================
resource "aws_ecs_service" "cca" {
  name            = "cca-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.cca.arn
  desired_count   = var.cca_service_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.cca.arn
    container_name   = "credit-card-authorizer"
    container_port   = var.cca_service_port
  }

  depends_on = [
    aws_lb_listener.http
  ]
}