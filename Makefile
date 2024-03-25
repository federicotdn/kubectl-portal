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

package:
	@test -n "$$VERSION" || (echo "VERSION not set!" && exit 1)
	mkdir -p dist
	rm -f dist/*
	GOOS=darwin GOARCH=amd64 make package-build
	GOOS=darwin GOARCH=arm64 make package-build
	GOOS=linux GOARCH=amd64 make package-build
	GOOS=linux GOARCH=arm64 make package-build
	EXT=.exe GOOS=windows GOARCH=amd64 make package-build
	SHA256_DARWIN_AMD64=$$(sha256sum dist/kubectl-portal-darwin-amd64.tar.gz | cut -d ' ' -f 1) \
	SHA256_DARWIN_ARM64=$$(sha256sum dist/kubectl-portal-darwin-arm64.tar.gz | cut -d ' ' -f 1) \
	SHA256_LINUX_AMD64=$$(sha256sum dist/kubectl-portal-linux-amd64.tar.gz | cut -d ' ' -f 1) \
	SHA256_LINUX_ARM64=$$(sha256sum dist/kubectl-portal-linux-arm64.tar.gz | cut -d ' ' -f 1) \
	SHA256_WINDOWS_AMD64=$$(sha256sum dist/kubectl-portal-windows-amd64.tar.gz | cut -d ' ' -f 1) \
		envsubst < .krew.yaml | tee dist/portal.yaml

package-build:
	make build
	tar -czf dist/kubectl-portal-$$GOOS-$$GOARCH.tar.gz kubectl-portal$$EXT LICENSE

pre-push: fmt lint
