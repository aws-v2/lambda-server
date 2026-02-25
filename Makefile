.PHONY: build run test clean docker-build docker-run

BINARY_NAME=lambda-server
MAIN_PATH=cmd/api/main.go

build:
	go build -o $(BINARY_NAME) $(MAIN_PATH)

run:
	go run $(MAIN_PATH)

test:
	go test ./...

clean:
	go clean
	rm -f $(BINARY_NAME)

docker-build:
	docker build -t $(BINARY_NAME) .

docker-run:
	docker run -p 8053:8053 $(BINARY_NAME)
