default: fmt lint build docs

build: clean
	go build -v

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test: clean .testenv-container
	TF_ACC=1 go test -v -cover -timeout 120m ./...

testenv-image:
	podman build -t terraform-provider-ldap:latest test/

.testenv-container: testenv-image
	@echo "Starting test container..."
	podman run -d --rm -p 3389:1389 terraform-provider-ldap:latest > $@
	@echo "Container ID: $$(cat $@)"
	@echo "Waiting for container to be ready..."
	@sleep 5

clean:
	@if [ -f .testenv-container ]; then \
		echo "Stopping test container: $$(cat .testenv-container)"; \
		podman stop "$$(cat .testenv-container)" 2>/dev/null || true; \
		rm -f .testenv-container; \
	fi
	rm -f terraform-provider-ldap
	@echo "Cleaned up build and test artifacts"

.PHONY: fmt lint test build install docs testenv-image clean
