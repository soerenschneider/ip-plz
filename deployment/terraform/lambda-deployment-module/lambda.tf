locals {
  archive_file = "${path.module}/../../../ip-plz-lambda.zip"
}

resource "aws_lambda_function" "ip_plz" {
  architectures    = ["arm64"]
  function_name    = "ip_plz"
  filename         = local.archive_file
  source_code_hash = filebase64sha256(local.archive_file)
  handler          = "bootstrap"
  role             = aws_iam_role.ip_plz.arn
  runtime          = "provided.al2"
  memory_size      = 128
  timeout          = 1
}

resource "aws_iam_role" "ip_plz" {
  name               = "ip_plz"
  assume_role_policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": {
    "Action": "sts:AssumeRole",
    "Principal": {
      "Service": "lambda.amazonaws.com"
    },
    "Effect": "Allow"
  }
}
POLICY
}
