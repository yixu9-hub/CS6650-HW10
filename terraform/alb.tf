# =========================
# 文件：terraform/alb.tf
# 作用：Security Group + ALB + Target Groups + Listener + 规则
# =========================

# Security Group for ALB
resource "aws_security_group" "alb" {
  name        = "ecommerce-alb-sg"
  description = "Security group for Application Load Balancer"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTP from anywhere"
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTPS from anywhere"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "ecommerce-alb-sg"
  }
}

# Application Load Balancer
resource "aws_lb" "main" {
  name               = "ecommerce-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = var.subnet_ids

  enable_deletion_protection = false
  enable_http2               = true

  tags = {
    Name = "ecommerce-alb"
  }
}

# =========================
# Target Group for Product Service
# =========================
resource "aws_lb_target_group" "product" {
  name        = "product-service-tg"
  port        = var.product_service_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id

  # Fargate + awsvpc 必须用 ip
  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/products/1"
    matcher             = "200,404"
  }

  # Automatic target weights for managing bad instances
  load_balancing_algorithm_type = "weighted_random"

  tags = {
    Name = "product-service-tg"
  }
}

# =========================
# Target Group for Shopping Cart Service
# =========================
resource "aws_lb_target_group" "shopping_cart" {
  name        = "shopping-cart-service-tg"
  port        = var.shopping_cart_service_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id

  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30
    path                = "/shopping-cart"
    matcher             = "405"  # GET /shopping-cart → 405，也算活着
  }

  load_balancing_algorithm_type = "weighted_random"

  tags = {
    Name = "shopping-cart-service-tg"
  }
}

# =========================
# Target Group for Credit Card Authorizer Service
# =========================
resource "aws_lb_target_group" "cca" {
  name        = "cca-service-tg"
  port        = var.cca_service_port
  protocol    = "HTTP"
  vpc_id      = var.vpc_id

  target_type = "ip"

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 3
    timeout             = 5
    interval            = 30

    # ★ CHANGED: 直接打 OpenAPI 定义的路径
    path    = "/credit-card-authorizer/authorize"
    matcher = "200-499"  # GET 可能收到 400 / 405 / 200，统统当作 healthy
  }

  load_balancing_algorithm_type = "weighted_random"

  tags = {
    Name = "cca-service-tg"
  }
}

# Listener on port 80
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  # Default action - return 404
  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "application/json"
      message_body = "{\"error\": \"NOT_FOUND\", \"message\": \"Service not found\"}"
      status_code  = "404"
    }
  }
}

# Listener Rule for Product Service - path-based routing
resource "aws_lb_listener_rule" "product" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 100

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.product.arn
  }

  condition {
    path_pattern {
      values = ["/product*"]
    }
  }

  tags = {
    Name = "product-service-rule"
  }
}

# Listener Rule for Shopping Cart Service - path-based routing
resource "aws_lb_listener_rule" "shopping_cart" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 200

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.shopping_cart.arn
  }

  condition {
    path_pattern {
      values = ["/shopping-cart*"]
    }
  }

  tags = {
    Name = "shopping-cart-service-rule"
  }
}

# Listener Rule for Credit Card Authorizer Service - path-based routing
resource "aws_lb_listener_rule" "cca" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 300

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.cca.arn
  }

  condition {
    path_pattern {
      # ★ CHANGED: 匹配 /credit-card-authorizer 开头的所有路径
      values = ["/credit-card-authorizer*"]
    }
  }

  tags = {
    Name = "cca-service-rule"
  }
}