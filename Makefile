BINARY := tfplantui
CMD    := ./cmd/tfplantui

.PHONY: build install test

# compile the binary into ./tfplantui
build:
	go build -o $(BINARY) $(CMD)

# install tfplantui into $GOBIN (~/go/bin)
install:
	go install $(CMD)

# run the test suite
test:
	go test ./...
