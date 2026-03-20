.PHONY: build run test clean lint tidy demo demo-build conversation

BINARY=agent-social-gateway
CMD=./cmd/gateway

build:
	go build -o bin/$(BINARY) $(CMD)
	go build -o bin/agent ./cmd/agent

demo-build: build
	go build -o bin/demo-agents ./cmd/demo-agents

run: build
	./bin/$(BINARY) -config configs/gateway.yaml

demo: demo-build
	@echo "Starting gateway in background..."
	@rm -f gateway.db gateway.db-wal gateway.db-shm
	@./bin/$(BINARY) -config configs/gateway.yaml &
	@sleep 2
	./bin/demo-agents
	@-pkill -f "bin/$(BINARY)" 2>/dev/null

conversation: build
	@bash scripts/demo-conversation.sh

conversation-deepseek: build
	@bash scripts/demo-conversation.sh --llm deepseek --model deepseek-chat

conversation-openai: build
	@bash scripts/demo-conversation.sh --llm openai

test:
	go test -race -v ./...

clean:
	rm -rf bin/ gateway.db gateway.db-wal gateway.db-shm

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

dev:
	go run $(CMD) -config configs/gateway.yaml
