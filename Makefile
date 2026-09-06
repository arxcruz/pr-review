.PHONY: build test run clean install

BINARY_NAME=pr-review

build:
	go build -o $(BINARY_NAME) ./cmd/pr-review

test:
	go test -v ./...

run: build
	./$(BINARY_NAME)

install:
	go install ./cmd/pr-review

clean:
	rm -f $(BINARY_NAME)
