package main

import (
	"log"
	"github.com/nats-io/nats.go"
)

func main() {
	nc, err := nats.Connect("nats://auth-server:auth-secret@localhost:4222")
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	payload := `{"tenant_id":"test-tenant","function_id":"my-function","reason":"CPU high","metric":"cpu","value":90,"action":"INCREASE_PROVISIONED_CONCURRENCY"}`
	err = nc.Publish("dev.lambda.v1.scale.out", []byte(payload))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Published scale out event")
}
