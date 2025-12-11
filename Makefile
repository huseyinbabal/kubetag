.PHONY: build run test docker-build docker-run clean deploy

# Build the application
build:
	go build -o kubetag cmd/server/main.go

# Run the application locally
run:
	go run cmd/server/main.go

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t kubetag:latest .

# Run Docker container
docker-run: docker-build
	docker run -p 8080:8080 -v ${HOME}/.kube/config:/root/.kube/config kubetag:latest

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s/

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f k8s/

# Clean build artifacts
clean:
	rm -f kubetag

# Install dependencies
deps:
	go mod download

# Tidy dependencies
tidy:
	go mod tidy
