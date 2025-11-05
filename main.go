package main

import (
	"fmt"
	"log"

	"go_test/worker"
)

func main() {
	handler := worker.NewDefaultHandler()
	url := "https://petstore.swagger.io/v2/pet/findByStatus?status=available"

	if err := handler.Handle(url); err != nil {
		log.Fatalf("Error handling request: %v", err)
	}

	fmt.Println("Successfully completed request")
}
