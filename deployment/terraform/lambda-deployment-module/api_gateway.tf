data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

resource "aws_api_gateway_rest_api" "ip_plz" {
  name        = "ip_plz_api"
  description = "API Gateway for IP PLZ Lambda"
}

resource "aws_api_gateway_resource" "ip_plz" {
  rest_api_id = aws_api_gateway_rest_api.ip_plz.id
  parent_id   = aws_api_gateway_rest_api.ip_plz.root_resource_id
  path_part   = "ip"
}

resource "aws_api_gateway_method" "ip_plz" {
  rest_api_id   = aws_api_gateway_rest_api.ip_plz.id
  resource_id   = aws_api_gateway_resource.ip_plz.id
  http_method   = "GET"
  authorization = "NONE"
  request_parameters = { "method.request.header.X-Forwarded-For" = false }
}

resource "aws_api_gateway_method_settings" "path_specific" {
  rest_api_id            = aws_api_gateway_rest_api.ip_plz.id
  stage_name             = aws_api_gateway_stage.ip_plz_v1.stage_name
  method_path            = "*/*"

  settings {
    logging_level = "OFF"
    throttling_rate_limit  = 1
    throttling_burst_limit = 5
  }
}

resource "aws_api_gateway_integration" "ip_plz" {
  rest_api_id             = aws_api_gateway_rest_api.ip_plz.id
  resource_id             = aws_api_gateway_resource.ip_plz.id
  http_method             = aws_api_gateway_method.ip_plz.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.ip_plz.invoke_arn
  timeout_milliseconds    = 300
}

resource "aws_api_gateway_deployment" "ip_plz_v1" {
  depends_on = [
    aws_api_gateway_integration.ip_plz,
    aws_api_gateway_method.ip_plz
  ]
  rest_api_id = aws_api_gateway_rest_api.ip_plz.id
}

resource "aws_api_gateway_stage" "ip_plz_v1" {
  deployment_id = aws_api_gateway_deployment.ip_plz_v1.id
  rest_api_id   = aws_api_gateway_rest_api.ip_plz.id
  stage_name    = "v1"
}

resource "aws_lambda_permission" "api_gateway_permission" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ip_plz.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "arn:aws:execute-api:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:${aws_api_gateway_rest_api.ip_plz.id}/*/${aws_api_gateway_method.ip_plz.http_method}${aws_api_gateway_resource.ip_plz.path}"
}
