PKGS := $(shell go list ./... | grep -v /vendor/)
COMMIT := $(shell git rev-parse HEAD)

build:
	@echo "Compiling source"
	@mkdir -p bin
	@GOBIN="bin" go install -ldflags "-X main.revision=$(COMMIT)" $(PKGS)

clean:
	@echo "Cleaning up workspace"
	@rm -rf bin

test:
	@echo "Running tests"
	@for pkg in $(PKGS) ; do \
		golint $$pkg ; \
	done
	@go vet $(PKGS)
	@go test -v $(PKGS) -cover
