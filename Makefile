.PHONY: all build build-web build-go test test-coverage serve compile-sample clean

BINARY_NAME=runforge
ifeq ($(OS),Windows_NT)
	BINARY_EXT=.exe
else
	BINARY_EXT=
endif

all: build test

build: build-web build-go

build-web:
	@echo "--> Building React 19 Control Plane Studio..."
	cd web && npm install && npm run build
	@mkdir -p internal/server/dist
	cp -r web/dist/* internal/server/dist/ || xcopy /E /I /Y web\dist internal\server\dist

build-go:
	@echo "--> Compiling Zero-CGO Go Single Binary (<25MB target)..."
	go build -ldflags="-s -w" -o $(BINARY_NAME)$(BINARY_EXT) ./cmd/runforge
	@echo "--> Successfully generated $(BINARY_NAME)$(BINARY_EXT)"

test:
	@echo "--> Running test suite across all packages..."
	go test -v ./...

test-coverage:
	@echo "--> Running test coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

serve: build
	./$(BINARY_NAME)$(BINARY_EXT) serve --port 7777

compile-sample:
	./$(BINARY_NAME)$(BINARY_EXT) compile examples/sample_trajectory.json -o examples/templates/code_review_pipeline.yaml --script examples/templates/code_review.sh

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe runforge.db runforge.db-shm runforge.db-wal coverage.out
