output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = aws_lb.main.dns_name
}

output "alb_arn" {
  description = "ARN of the Application Load Balancer"
  value       = aws_lb.main.arn
}

output "product_target_group_arn" {
  description = "ARN of the product service target group"
  value       = aws_lb_target_group.product.arn
}

output "shopping_cart_target_group_arn" {
  description = "ARN of the shopping cart service target group"
  value       = aws_lb_target_group.shopping_cart.arn
}

output "cca_target_group_arn" {
  description = "ARN of the credit card authorizer service target group"
  value       = aws_lb_target_group.cca.arn
}
