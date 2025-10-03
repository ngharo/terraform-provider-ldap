default: fmt lint build docs

build: clean
	go build -v

install: build
	go install -v ./...

lint:
	golangci-lint run

docs:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test: clean testenv
	TF_ACC=1 go test -v -cover -timeout 120m ./...

testenv: .testenv-container-id
	podman build -t terraform-provider-ldap:latest test/

.testenv-container-id:
	@echo "Starting test container..."
	podman run -d --rm -p 3389:1389 terraform-provider-ldap:latest > $@
	@echo "Container ID: $$(cat $@)"
	@echo "Waiting for container to be ready..."
	@sleep 5

clean:
	@if [ -f .testenv-container-id ]; then \
		echo "Stopping test container: $$(cat .test-container-id)"; \
		podman stop "$$(cat .test-container-id)" 2>/dev/null || true; \
		rm -f .testenv-container-id; \
	fi
	rm -f terraform-provider-ldap
	@echo "Cleaned up build and test artifacts"

.PHONY: fmt lint test build install docs testenv clean
