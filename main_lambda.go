//go:build aws

package main

import (
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

const forwardedForHeader = "X-Forwarded-For"

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	forwardedForValue := request.Headers[forwardedForHeader]
	for _, ip := range strings.Split(forwardedForValue, ",") {
		ip, err := GetPublicIp(ip)
		if err == nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 200,
				Body:       ip,
			}, nil
		}
	}

	return events.APIGatewayProxyResponse{
		Body:       fmt.Sprintf("Missing '%s' forwardedForHeader", forwardedForHeader),
		StatusCode: 400,
	}, nil
}

func main() {
	lambda.Start(handler)
}
