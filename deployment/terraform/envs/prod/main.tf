module "ip-plz" {
  source = "../../lambda-deployment-module"
}

output "prometheus_token" {
  value = module.ip-plz.api_gateway_invoke_url
}
