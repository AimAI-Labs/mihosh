BINARY ?= mihosh
VERSION ?= dev
COMMIT ?= unknown
DATE ?= unknown
LDFLAGS = -ldflags "-s -w -X github.com/AimAI-Labs/mihosh/internal/domain/model.Version=$(VERSION) -X github.com/AimAI-Labs/mihosh/internal/domain/model.Commit=$(COMMIT) -X github.com/AimAI-Labs/mihosh/internal/domain/model.Date=$(DATE)"

.PHONY: fmt vet test arch-check build check clean

fmt:
	go fmt ./...

vet:
	go vet ./...

arch-check:
	go test -v ./internal -run TestArchitectureBoundaries

test:
	go test ./...

build:
	go build $(LDFLAGS) -o $(BINARY) .

check: fmt vet arch-check test build

clean:
	$(RM) $(BINARY) $(BINARY).exe coverage.out
