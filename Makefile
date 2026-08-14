APP := droidforge
PACKAGE := ./cmd/droidforge
DIST := dist

.PHONY: build test release clean

build:
	go build -o bin/$(APP) $(PACKAGE)

test:
	go test $(PACKAGE)
	go vet $(PACKAGE)

release:
	mkdir -p $(DIST)
	GOOS=darwin GOARCH=amd64 go build -o $(DIST)/$(APP)-darwin-amd64 $(PACKAGE)
	GOOS=darwin GOARCH=arm64 go build -o $(DIST)/$(APP)-darwin-arm64 $(PACKAGE)
	GOOS=linux GOARCH=amd64 go build -o $(DIST)/$(APP)-linux-amd64 $(PACKAGE)
	GOOS=linux GOARCH=arm64 go build -o $(DIST)/$(APP)-linux-arm64 $(PACKAGE)

clean:
	rm -rf bin $(DIST)
