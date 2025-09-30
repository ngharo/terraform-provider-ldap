default: fmt lint install generate

build: clean
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc: clean testcontainer-run
	TF_ACC=1 go test -v -cover -timeout 120m ./...

testcontainer:
	podman build -t terraform-provider-ldap:latest test/

testcontainer-run: testcontainer .test-container-id

.test-container-id:
	@echo "Starting test container..."
	podman run -d --rm -p 3389:1389 terraform-provider-ldap:latest > $@
	@echo "Container ID: $$(cat $@)"
	@echo "Waiting for container to be ready..."
	@sleep 5

clean:
	@if [ -f .test-container-id ]; then \
		echo "Stopping test container: $$(cat .test-container-id)"; \
		podman stop "$$(cat .test-container-id)" 2>/dev/null || true; \
		rm -f .test-container-id; \
	fi
	rm -f terraform-provider-ldap
	@echo "Cleaned up build and test artifacts"

.PHONY: fmt lint test testacc build install generate testcontainer testcontainer-run clean
