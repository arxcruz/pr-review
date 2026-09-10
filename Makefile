.PHONY: build test run clean install

BINARY_NAME=pr-review
JIRA_BINARY_NAME=jira-refine

build:
	go build -o $(BINARY_NAME) ./cmd/pr-review
	go build -o $(JIRA_BINARY_NAME) ./cmd/jira-refine

test:
	go test -v ./...

run: build
	./$(BINARY_NAME)

install:
	go install ./cmd/pr-review
	go install ./cmd/jira-refine

clean:
	rm -f $(BINARY_NAME) $(JIRA_BINARY_NAME)
