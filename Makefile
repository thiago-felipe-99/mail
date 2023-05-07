.PHONY: docs_publisher
docs_publisher:
	cd ./publisher/ && swag init

.PHONY: lint
lint:
	swag fmt ./publisher/
	golangci-lint run --fix ./consumer/... ./publisher/... ./rabbit/...

.PHONEY: tidy_consumer
tidy_consumer:
	cd ./consumer/ && go mod tidy

.PHONEY: tidy_publisher
tidy_publisher:
	cd ./publisher/ && go mod tidy

.PHONEY: tidy_rabbit
tidy_rabbit:
	cd ./rabbit/ && go mod tidy

.PHONY: tidy
tidy: tidy_consumer tidy_publisher tidy_rabbit
	go mod tidy

.PHONY: run_consumer
run_consumer: tidy_consumer
	golangci-lint run --fix ./consumer/...
	go run ./consumer/

.PHONY: run_publisher
run_publisher: docs_publisher tidy_publisher
	swag fmt ./publisher/
	golangci-lint run --fix ./publisher/...
	go run ./publisher/

.PHONY: all
all: docs_publisher lint tidy
