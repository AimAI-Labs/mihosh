BINARY ?= mihosh
LDFLAGS = -ldflags "-s -w -X github.com/AimAI-Labs/mihosh/internal/domain/model.Version=$(VERSION) -X github.com/AimAI-Labs/mihosh/internal/domain/model.Commit=$(COMMIT) -X github.com/AimAI-Labs/mihosh/internal/domain/model.Date=$(DATE)"

.PHONY: fmt vet test build check clean

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	go build $(LDFLAGS) -o $(BINARY) .

check: fmt vet test build

clean:
	$(RM) $(BINARY) $(BINARY).exe coverage.out
