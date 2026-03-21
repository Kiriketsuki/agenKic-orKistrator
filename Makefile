.PHONY: generate test test-integration test-e2e lint build clean

BINARY_NAME=orchestrator
BINARY_OUT=bin/$(BINARY_NAME)
PROTO_DIR=proto
GEN_DIR=gen/pb

generate:
	mkdir -p $(GEN_DIR)/orchestrator
	protoc \
		--go_out=. \
		--go_opt=module=github.com/Kiriketsuki/agenKic-orKistrator \
		--go-grpc_out=. \
		--go-grpc_opt=module=github.com/Kiriketsuki/agenKic-orKistrator \
		$(PROTO_DIR)/orchestrator.proto

test:
	go test -race -count=1 -tags=testenv ./internal/...

test-integration:
	go test -race -count=1 -tags=integration ./internal/...

test-e2e:
	go test -race -count=1 -timeout=60s -tags=testenv ./e2e/...

lint:
	golangci-lint run ./...

build:
	mkdir -p bin
	go build -o $(BINARY_OUT) ./cmd/orchestrator

clean:
	rm -rf bin/ $(GEN_DIR)/
