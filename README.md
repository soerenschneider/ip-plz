# ip-plz
[![Go Report Card](https://goreportcard.com/badge/github.com/soerenschneider/ip-plz)](https://goreportcard.com/report/github.com/soerenschneider/ip-plz)
![test-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/test.yaml/badge.svg)
![release-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/release-container.yaml/badge.svg)
![golangci-lint-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/golangci-lint.yaml/badge.svg)

ip-plz is a tiny HTTP API that returns caller's public IP address

## Features

✉️ Returns the caller's **public** IP<br/>
🔀 Reverse-proxy support with configurable trusted headers<br/>
🚀 Deploy as container, AWS lambda or traditional binary<br/>
📦 Zero configuration and works out of the box with secure defaults<br/>
🪶 Lightweight and almost no dependencies<br/>
🔭 Observability using metrics<br/>

## Installation

### AWS Lambda
Run the `lambda` Makefile target to build an archive for the `provided.al2` runtime and the `arm64` architecture. Either
create a Lambda using AWS cli / Console or run the Terraform code to create an API Gateway instance and Lambda function.

```shell
$ make lambda
$ terraform -chdir=deployment/terraform/envs/dev apply
$ curl $(terraform -chdir=tf output --raw api_gateway_invoke_url)v1/ip
```

### Docker / Podman
```shell
$ docker run -d -p 8080:8080 ghcr.io/soerenschneider/ip-plz
$ curl localhost:8080/ip-plz
```

### Binaries
Head to the [releases section](https://github.com/soerenschneider/ip-plz/releases) and download the correct binary for your system.

### From Source
As a prerequisite, you need to have [Golang SDK](https://go.dev/dl/) installed. After that, you can install ip-plz from source by invoking:
```text
$ go install github.com/soerenschneider/ip-plz@latest
```

## Example
```shell
$ curl https://dtp9iioxbc.execute-api.us-east-1.amazonaws.com/prod/ip
178.200.50.73
```

## Configuration

Configuration is only available when not deploying via Lambda. ip-plz is configured via environment variables only.

Configuration reference:

| Field Name        | Type      | Environment Variable       | Description                                                                                              | Default Value |
|-------------------|-----------|----------------------------|----------------------------------------------------------------------------------------------------------|---------------|
| MetricsAddr       | string    | IP_PLZ_METRICS_ADDR        | The address for serving metrics.                                                                         | ":9191"       |
| Path              | string    | IP_PLZ_PATH                | The path where the service is available.                                                                 | "/ip-plz"     |
| Address           | string    | IP_PLZ_ADDR                | The network address to bind the service to.                                                              | ":8080"       |
| TrustedHeaders    | []string  | IP_PLZ_TRUSTED_HEADERS     | A list of trusted HTTP headers (comma-separated in the environment).                                     | -             |
| ReadTimeout       | int       | IP_PLZ_READ_TIMEOUT        | Maximum duration for reading the entire request, in seconds.                                             | 1 second      |
| WriteTimeout      | int       | IP_PLZ_WRITE_TIMEOUT       | Maximum duration for writing the response back to the client, in seconds.                                | 1 second      |
| IdleTimeout       | int       | IP_PLZ_IDLE_TIMEOUT        | Maximum duration the server should wait for the next request when no connections are active, in seconds. | 5 seconds     |
| ReadHeaderTimeout | int       | IP_PLZ_READ_HEADER_TIMEOUT | Maximum duration for reading the request headers, in seconds.                                            | 2 seconds     |

## Metrics

Metrics are prefixed with `ip_plz`

| Metric Name                             | Description                                   | Metric Type |
|-----------------------------------------|-----------------------------------------------|-------------|
| `requests_total`                        | The total number of processed requests        | Counter     |
| `most_recent_request_timestamp_seconds` | Timestamp of the most recent request received | Gauge       |


## Changelog
Read the [full changelog](CHANGELOG.md)
