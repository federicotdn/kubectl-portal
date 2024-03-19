SHELL = bash
GOLANGCI_LINT_CACHE = ~/.cache/golangci-lint/latest
TOOL_DIR = ./cmd/kubectl-portal
PROXY_DIR = ./cmd/kubectl-portal-proxy

build:
	cp go.mod $(TOOL_DIR)/data/go.mod.copy
	cp go.sum $(TOOL_DIR)/data/go.sum.copy
	cp $(PROXY_DIR)/main.go $(TOOL_DIR)/data
	go build $(TOOL_DIR)

build-proxy:
	go build $(PROXY_DIR)

clean:
	rm -f kubectl-portal \
		  kubectl-portal-proxy \
		  $(TOOL_DIR)/data/*
	sudo rm -rf $(GOLANGCI_LINT_CACHE)

fmt:
	gofmt -s -w -l cmd

checkfmt:
	test -z "$$(gofmt -l cmd)"

lint:
	docker run -t --rm -v $$(pwd):/app -v $(GOLANGCI_LINT_CACHE):/root/.cache -w /app golangci/golangci-lint:latest golangci-lint run -v

pre-push: fmt lint
