# ip-plz
[![Go Report Card](https://goreportcard.com/badge/github.com/soerenschneider/ip-plz)](https://goreportcard.com/report/github.com/soerenschneider/ip-plz)
![test-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/test.yaml/badge.svg)
![release-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/release-container.yaml/badge.svg)
![golangci-lint-workflow](https://github.com/soerenschneider/ip-plz/actions/workflows/golangci-lint.yaml/badge.svg)

ip-plz is a tiny HTTP API that returns caller's public IP address

## Features

✉️ Returns the public IP the HTTP request came from
🔀 Reverse-proxy support with configurable allowed headers
🪽 Lightweight and almost no dependencies

## Configuration

ip-plz is configured via environment variables only. The following configuration reference shows configurable options.

| Field Name        | Type      | Environment Variable       | Description                                                                                             | Default Value |
|-------------------|-----------|----------------------------|---------------------------------------------------------------------------------------------------------|---------------|
| MetricsAddr       | string    | IP_PLZ_METRICS_ADDR        | The address for serving metrics.                                                                        | ":9191"       |
| Path              | string    | IP_PLZ_PATH                | The path where the service is available.                                                                | "/ip-plz"     |
| Address           | string    | IP_PLZ_ADDR                | The network address to bind the service to.                                                             | ":8080"       |
| TrustedHeaders    | []string  | IP_PLZ_TRUSTED_HEADERS     | A list of trusted HTTP headers (comma-separated in the environment).                                    | -             |
| ReadTimeout       | int       | IP_PLZ_READ_TIMEOUT        | Maximum duration for reading the entire request, in seconds.                                            | 1 second      |
| WriteTimeout      | int       | IP_PLZ_WRITE_TIMEOUT       | Maximum duration for writing the response back to the client, in seconds.                               | 1 second      |
| IdleTimeout       | int       | IP_PLZ_IDLE_TIMEOUT        | Maximum duration the server should wait for the next request when no connections are active, in seconds. | 5 seconds     |
| ReadHeaderTimeout | int       | IP_PLZ_READ_HEADER_TIMEOUT | Maximum duration for reading the request headers, in seconds.                                           | 2 seconds     |


## Metrics

Metrics are prefixed with `ip_plz`

| Metric Name                             | Description                                   | Metric Type |
|-----------------------------------------|-----------------------------------------------|-------------|
| `requests_total`                        | The total number of processed requests        | Counter     |
| `most_recent_request_timestamp_seconds` | Timestamp of the most recent request received | Gauge       |


## Changelog
Read the [full changelog](CHANGELOG.md)
