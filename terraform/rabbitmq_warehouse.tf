# =========================================
# 文件：terraform/rabbitmq_warehouse.tf
# 作用：在 ECS Fargate 上部署 RabbitMQ + Warehouse Consumer
# =========================================

# NEW: Security Group for RabbitMQ
resource "aws_security_group" "rabbitmq" {
  name        = "ecommerce-rabbitmq-sg"
  description = "Security group for RabbitMQ broker"
  vpc_id      = var.vpc_id

  # 允许任何来源访问 5672（AMQP）和 15672（管理界面）
  ingress {
    from_port   = 5672
    to_port     = 5672
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow AMQP traffic"
  }

  ingress {
    from_port   = 15672
    to_port     = 15672
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow RabbitMQ management UI"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name = "ecommerce-rabbitmq-sg"
  }
}

# =========================================
# NEW: RabbitMQ Task Definition
# 使用 rabbitmq:3-management 镜像（带 Web UI）
# =========================================
resource "aws_ecs_task_definition" "rabbitmq" {
  family                   = "rabbitmq-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "rabbitmq"
      image = var.rabbitmq_service_image   # NEW: 来自 variables.tf
      portMappings = [
        {
          containerPort = 5672
          hostPort      = 5672
          protocol      = "tcp"
        },
        {
          containerPort = 15672
          hostPort      = 15672
          protocol      = "tcp"
        }
      ]
      essential = true
      environment = [
        {
          name  = "RABBITMQ_DEFAULT_USER"
          value = "guest"
        },
        {
          name  = "RABBITMQ_DEFAULT_PASS"
          value = "guest"
        }
      ]
    }
  ])
}

# =========================================
# NEW: RabbitMQ ECS Service
# 对外直接用 Public IP + 15672 看控制台
# =========================================
resource "aws_ecs_service" "rabbitmq" {
  name            = "rabbitmq-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.rabbitmq.arn
  desired_count   = var.rabbitmq_service_desired_count  # NEW
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = var.subnet_ids
    security_groups = [aws_security_group.rabbitmq.id]
    assign_public_ip = true
  }

  depends_on = [
    aws_lb.main  # 确保 VPC / Subnets 等资源先创建
  ]
}

# =========================================
# NEW: Warehouse Consumer Task Definition
# 消费 RabbitMQ 队列，进行统计
# =========================================
resource "aws_ecs_task_definition" "warehouse" {
  family                   = "warehouse-consumer-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.labrole.arn
  task_role_arn            = data.aws_iam_role.labrole.arn

  container_definitions = jsonencode([
    {
      name  = "warehouse-consumer"
      image = var.warehouse_service_image   # NEW
      portMappings = []                     # 通常不对外暴露端口
      essential = true
      environment = [
        {
          name  = "RABBITMQ_URI"
          value = var.rabbitmq_uri          # NEW: 队列连接 URI
        },
        {
          name  = "WAREHOUSE_WORKERS"
          value = tostring(var.warehouse_workers)   # NEW: worker 数量
        }
      ]

      # ★★★ 关键：打开 awslogs 日志 ★★★
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.warehouse.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}

# =========================================
# NEW: Warehouse ECS Service
# 跟其他服务一样跑在 Fargate 上
# =========================================
resource "aws_ecs_service" "warehouse" {
  name            = "warehouse-consumer-service"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.warehouse.arn
  desired_count   = var.warehouse_service_desired_count  # NEW
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = var.subnet_ids
    security_groups = [aws_security_group.ecs_service.id]
    assign_public_ip = true
  }

  # 可以依赖 RabbitMQ service 先起来
  depends_on = [
    aws_ecs_service.rabbitmq
  ]
}


resource "aws_cloudwatch_log_group" "warehouse" {
  name              = "/ecs/warehouse-consumer"
  retention_in_days = 3

  tags = {
    Name = "warehouse-consumer-logs"
  }
}