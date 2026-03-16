.PHONY: all build test clean docker docker-push deploy help

# Variables
IMAGE_NAME ?= yourorg/dbf
IMAGE_TAG ?= v0.1.0
NAMESPACE ?= dbf

# Go variables
GO := go
GOFLAGS := -v
LDFLAGS := -ldflags="-w -s"

all: build test

## build: Build the server binary
build:
	@echo "Building server..."
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/server ./cmd/server
	@echo "Building client..."
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/client ./cmd/client

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem -benchtime=1s ./pkg/bloom

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f client server

## docker: Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(IMAGE_NAME):latest

## docker-push: Push Docker image to registry
docker-push:
	@echo "Pushing Docker image..."
	docker push $(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(IMAGE_NAME):latest

## deploy: Deploy to Kubernetes
deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deploy/k8s/namespace.yaml
	kubectl apply -f deploy/k8s/configmap.yaml
	kubectl apply -f deploy/k8s/pvc.yaml
	kubectl apply -f deploy/k8s/statefulset.yaml
	kubectl apply -f deploy/k8s/deployment.yaml
	kubectl apply -f deploy/k8s/service.yaml
	kubectl apply -f deploy/k8s/hpa.yaml
	kubectl apply -f deploy/k8s/pdb.yaml
	kubectl apply -f deploy/k8s/monitoring.yaml

## deploy-kustomize: Deploy using kustomize
deploy-kustomize:
	@echo "Deploying with kustomize..."
	kubectl apply -k deploy/k8s/

## undeploy: Remove from Kubernetes
undeploy:
	@echo "Undeploying from Kubernetes..."
	kubectl delete -k deploy/k8s/ --ignore-not-found=true
	kubectl delete namespace $(NAMESPACE) --ignore-not-found=true

## status: Check deployment status
status:
	@echo "Checking deployment status..."
	kubectl get pods -n $(NAMESPACE)
	kubectl get svc -n $(NAMESPACE)
	kubectl get statefulset -n $(NAMESPACE)
	kubectl get deployment -n $(NAMESPACE)

## logs: Follow logs
logs:
	kubectl logs -n $(NAMESPACE) -l app=distributed-bloom-filter -f

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	$(GO) mod tidy

## ci: Run CI pipeline
ci: fmt vet test lint build

## help: Show this help message
help:
	@echo "Distributed Bloom Filter - Makefile Commands"
	@echo "============================================="
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
