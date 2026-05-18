BINARY ?= mihosh

.PHONY: fmt vet test build check clean

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -o $(BINARY) .

check: fmt vet test build

clean:
	$(RM) $(BINARY) $(BINARY).exe coverage.out
