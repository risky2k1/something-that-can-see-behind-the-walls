BIN := bin

.PHONY: all build windows test vet fmt clean

all: fmt vet test build

build:
	go build -o $(BIN)/procsearch ./cmd/procsearch

windows:
	GOOS=windows GOARCH=amd64 go build -o $(BIN)/procsearch.exe ./cmd/procsearch

test:
	go test ./...

vet:
	go vet ./...
	GOOS=windows go vet ./...
	GOOS=darwin go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf $(BIN)
