# CS6650 – HW10: Microservice Extravaganza

Team Repository: https://github.com/yixu9-hub/CS6650-HW10

This project implements a simple e-commerce workflow using multiple microservices, an AWS Application Load Balancer (ALB), RabbitMQ, and a Warehouse consumer. It supports local development via Docker Compose and AWS deployment on ECS Fargate.

---

## 1. System Architecture

Local Client / Load Tester (curl / script)  
    ↓ HTTP  

AWS Application Load Balancer (ALB) – URL Path–Based Routing  
    ↓  
    /product*                 → Product Service Target Group  
    /shopping-cart*           → Shopping Cart Service Target Group  
    /credit-card-authorizer*  → CCA Service Target Group  

Shopping Cart Service  
    ↓ HTTP internal call  
Credit Card Authorizer (CCA)  
    ↓ on successful payment  
RabbitMQ Queue  
    ↓ AMQP  
Warehouse Consumer (multi-threaded, manual ACK)  

---

## 2. Microservices Overview

### 2.1 Product Service (`product-service`)

- `POST /product` – create a new product  
- `GET /products/{productId}` – fetch product details  
- Deployed locally (Docker Compose) and on ECS behind the Product Target Group

### 2.2 Bad Product Service (`product-service-bad`)

- Same API as Product Service  
- Returns HTTP `503` on roughly 50% of requests to simulate a flaky instance  
- Registered in the same ALB Target Group as the good Product Service  
- Used to demonstrate that the ALB with `weighted_random` sends less traffic to bad instances

### 2.3 Shopping Cart Service (`shopping-cart-service`)

Endpoints:

- `POST /shopping-cart` – create a new cart  
- `POST /shopping-carts/{shoppingCartId}/addItem` – add an item to the cart  
- `POST /shopping-carts/{shoppingCartId}/checkout` – perform checkout  

Checkout flow:

1. Validate the cart ID and body payload  
2. Ensure the cart is not empty  
3. Call CCA to authorise the credit card  
4. If payment is authorised, publish an order message to RabbitMQ  
5. Clear the cart and return an `order_id`  

### 2.4 Credit Card Authorizer (`credit-card-authorizer`)

- `POST /credit-card-authorizer/authorize` (local)  
- `POST /credit-card-authorizer/authorize` via ALB  
- Validates credit card format: `dddd-dddd-dddd-dddd`  
- Behaviour (matching the OpenAPI assignment spec):  
  - `400 Bad Request` – invalid JSON or bad card format  
  - `200 OK` – payment authorised  
  - `402 Payment Required` – payment declined (about 10% of valid cards, chosen randomly)  

### 2.5 RabbitMQ + Warehouse Consumer

- Shopping Cart Service publishes an `OrderMessage` to RabbitMQ after successful payment  
- The Warehouse Consumer subscribes to the queue using multiple worker goroutines (configured by `WAREHOUSE_WORKERS`)  
- Uses manual consumer acknowledgements  
- Maintains:  
  - total number of orders processed  
  - per-product quantity totals (in memory)  
- On shutdown, logs the total number of processed orders  

---

## 3. Local Development (Docker Compose)

In the project root, `docker-compose.yml` starts the HTTP microservices; `src/docker-compose.yml` starts RabbitMQ + SCS + CCA + Warehouse for local end-to-end tests.

### 3.1 Start all local services

    docker-compose up --build

This starts:

- Product Service on `localhost:8080`  
- Shopping Cart Service on `localhost:8081`  
- Credit Card Authorizer on `localhost:8082`  
- Bad Product Service on `localhost:8083`  

(When using the `src/docker-compose.yml` file, RabbitMQ and the Warehouse Consumer are also started.)

---

## 4. Local Test Flow (matches the original assignment example)

### 4.1 Create product

    curl -X POST http://localhost:8080/product \
      -H "Content-Type: application/json" \
      -d '{"sku":"ABC123","manufacturer":"Acme","category_id":1,"weight":100,"some_other_id":1}'

Expected: `201 Created` with `{"product_id":1}`.

### 4.2 Get product

    curl http://localhost:8080/products/1

### 4.3 Create shopping cart

    curl -X POST http://localhost:8081/shopping-cart \
      -H "Content-Type: application/json" \
      -d '{"customer_id":1}'

Expected: `{"shopping_cart_id":1}`.

### 4.4 Add item to cart

    curl -X POST http://localhost:8081/shopping-carts/1/addItem \
      -H "Content-Type: application/json" \
      -d '{"product_id":1,"quantity":2}'

Expected: `204 No Content`.

### 4.5 Checkout

    curl -X POST http://localhost:8081/shopping-carts/1/checkout \
      -H "Content-Type: application/json" \
      -d '{"credit_card_number":"1234-5678-9012-3456"}'

Expected:

- `200 OK` – order accepted and message sent to RabbitMQ  
- `402 Payment Required` – payment declined (10% of time)  
- `400 Bad Request` – invalid card format or empty cart  

### 4.6 Test the bad Product Service (50% will return 503)

    for i in {1..10}; do
      curl -X POST http://localhost:8083/product \
        -H "Content-Type: application/json" \
        -d '{"sku":"TEST'"$i"'","manufacturer":"Test","category_id":1,"weight":100,"some_other_id":1}'
      echo ""
    done

---

## 5. Terraform / AWS Deployment

Terraform configuration is under `terraform/`:

- `main.tf` – provider configuration  
- `alb.tf` – ALB, Security Group, Target Groups, Listener + path routing rules  
- `ecs.tf` – ECS Cluster, Task Definitions, Services (Product, Product Bad, SCS, CCA, Warehouse)  
- `ecr.tf` – ECR repositories for each container image  
- `rabbitmq_warehouse.tf` – RabbitMQ broker and Warehouse consumer configuration (if provisioned via Terraform)  
- `variables.tf` – input variables (region, VPC ID, subnets, image URIs, desired counts, etc.)  
- `outputs.tf` – outputs (ALB DNS name, target group ARNs, ECR URLs, etc.)  
- `terraform.tfvars` – concrete values for this deployment (VPC ID, Subnet IDs, ECR URLs, etc.)

### 5.1 Prerequisites

- Terraform >= 1.6  
- AWS CLI configured for account `533267264147`  
- Docker with buildx and linux/amd64 support  

### 5.2 Build and push images (example: CCA)

    aws ecr get-login-password --region us-west-2 | \
      docker login --username AWS \
      --password-stdin 533267264147.dkr.ecr.us-west-2.amazonaws.com

    cd src/credit-card-authorizer
    docker buildx build --platform linux/amd64 \
      -t 533267264147.dkr.ecr.us-west-2.amazonaws.com/credit-card-authorizer:latest \
      . --push

Repeat similar commands for:

- `product-service`  
- `product-service-bad`  
- `shopping-cart-service`  
- `warehouse-consumer`  

(Repository names and URLs are defined in `terraform/ecr.tf`.)

### 5.3 Apply Terraform

    cd terraform
    terraform init
    terraform apply

Important outputs:

- `alb_dns_name` – ALB DNS used in all remote tests  
- `ecr_urls` – ECR repository URLs  

---

## 6. AWS End-to-End Tests via ALB

Set the ALB DNS name (replace with your actual output):

    export ALB="ecommerce-alb-XXXXX.us-west-2.elb.amazonaws.com"

### 6.1 Create product via ALB

    curl -v -X POST "http://$ALB/product" \
      -H "Content-Type: application/json" \
      -d '{"sku":"ABC123","manufacturer":"Acme","category_id":1,"weight":100,"some_other_id":1}'

### 6.2 Get product via ALB

    curl -v "http://$ALB/products/1"

### 6.3 Create cart

    curl -v -X POST "http://$ALB/shopping-cart" \
      -H "Content-Type: application/json" \
      -d '{"customer_id":1}'

### 6.4 Add item

    curl -v -X POST "http://$ALB/shopping-carts/1/addItem" \
      -H "Content-Type: application/json" \
      -d '{"product_id":1,"quantity":2}'

### 6.5 Checkout (SCS → CCA → RabbitMQ → Warehouse)

    curl -v -X POST "http://$ALB/shopping-carts/1/checkout" \
      -H "Content-Type: application/json" \
      -d '{"credit_card_number":"1234-5678-9012-3456"}'

---

## 7. Demonstrating ALB Handling of “Bad” Instances

To demonstrate that the ALB sends less traffic to the bad Product Service:

1. Run a small load test against `/product` via ALB:

       for i in {1..200}; do
         curl -s -o /dev/null -w "%{http_code}\n" \
           -X POST "http://$ALB/product" \
           -H "Content-Type: application/json" \
           -d "{\"sku\":\"T$i\",\"manufacturer\":\"Test\",\"category_id\":1,\"weight\":100,\"some_other_id\":1}"
       done

2. Open AWS Console → EC2 → Target Groups → `product-service-tg` → Monitoring.

3. Look at metrics grouped by target:
   - RequestCount per target
   - HTTPCode_Target_5XX_Count

You should see:
- The bad instance has many more 5XX responses
- Over time, ALB sends fewer requests to the bad instance compared to the healthy ones

These charts are used in the PDF report.

---

## 8. RabbitMQ and Warehouse Metrics

During high load:

- Shopping Cart Service will enqueue many order messages to RabbitMQ  
- Warehouse Consumer runs multiple workers (configured by `WAREHOUSE_WORKERS`)  
- Goal: keep queue length from growing without bound (no sharp “/\/\/\” spikes)  

Using the RabbitMQ management console, we track:

- Queue length over time  
- Publish rate and consume rate  

On shutdown, Warehouse logs:

    Total orders processed: N

This value plus the queue graphs should be included in the report.

---

## 9. Repository Structure

    CS6650-HW10/
    ├─ src/
    │  ├─ product-service/
    │  ├─ product-service-bad/
    │  ├─ shopping-cart-service/
    │  ├─ credit-card-authorizer/
    │  ├─ warehouse-consumer/
    │  └─ docker-compose.yml
    ├─ terraform/
    │  ├─ alb.tf
    │  ├─ ecs.tf
    │  ├─ ecr.tf
    │  ├─ rabbitmq_warehouse.tf
    │  ├─ main.tf
    │  ├─ outputs.tf
    │  ├─ variables.tf
    │  ├─ terraform.tfvars
    │  └─ terraform.tfvars.example (optional)
    └─ README.md

---

## 10. Authors

- Junyao Han, Yi Xu, Jiaming Pei
- CS6650 – HW10 Microservice Extravaganza