SHELL = bash
GOLANGCI_LINT_CACHE = ~/.cache/golangci-lint/latest

build:
	go build

clean:
	rm -rf kubectl-portal
	sudo rm -rf $(GOLANGCI_LINT_CACHE)

fmt:
	gofmt -s -w -l .

checkfmt:
	test -z "$$(gofmt -l .)"

run: build
	./kubectl-portal

lint:
	docker run -t --rm -v $$(pwd):/app -v $(GOLANGCI_LINT_CACHE):/root/.cache -w /app golangci/golangci-lint:latest golangci-lint run -v

pre-push: fmt lint
