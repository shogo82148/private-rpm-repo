package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handleEvent(ctx context.Context, event events.S3Event) (string, error) {
	for _, record := range event.Records {
		data, err := json.Marshal(record)
		if err != nil {
			return "", err
		}
		log.Println(string(data))
	}
	return "Hello ƛ!", nil
}

func main() {
	lambda.Start(handleEvent)
}
