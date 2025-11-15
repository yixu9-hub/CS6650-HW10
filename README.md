Local Load Testing Client
    ↓ HTTP
【AWS Application Load Balancer】
    ↓ URL-based routing
    ├─ /product/* → Product Service Target Group
    ├─ /shopping-cart/* → Shopping Cart Service Target Group  
    └─ /credit-card-authorizer/* → CCA Target Group
    
【Shopping Cart Service】
    ↓ (validates productId via Product Service? ← Your confusion!)
    ↓ (authorizes payment via CCA)
    ↓ AMQP
【RabbitMQ Queue】
    ↓ AMQP
【Warehouse Consumer Service】

# 启动所有服务
docker-compose up --build

测试：
# 1. 创建产品
curl -X POST http://localhost:8080/product -H "Content-Type: application/json" -d '{"sku":"ABC123","manufacturer":"Acme","category_id":1,"weight":100,"some_other_id":1}'

# 2. 获取产品
curl http://localhost:8080/products/1

# 3. 创建购物车
curl -X POST http://localhost:8081/shopping-cart -H "Content-Type: application/json" -d '{"customer_id":1}'

# 4. 添加商品到购物车
curl -X POST http://localhost:8081/shopping-carts/1/addItem -H "Content-Type: application/json" -d '{"product_id":1,"quantity":2}'

# 5. 结账
curl -X POST http://localhost:8081/shopping-carts/1/checkout -H "Content-Type: application/json" -d '{"credit_card_number":"1234-5678-9012-3456"}'

# 6. 测试坏的产品服务（50%会返回503）
for i in {1..10}; do curl -X POST http://localhost:8083/product -H "Content-Type: application/json" -d '{"sku":"TEST'$i'","manufacturer":"Test","category_id":1,"weight":100,"some_other_id":1}'; echo ""; done