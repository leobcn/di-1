GO ?= $(shell command -v go)
GO17 = /usr/local/go1.7.1
GO16 = /usr/local/go1.6.3
GO15 = /usr/local/go1.5.4
TESTFLAGS = -race -cover

test: tip go17 go16 go15

tip:
	$(GO) test $(TESTFLAGS) ./...

go17:
	GOROOT=$(GO17) $(GO17)/bin/go test $(TESTFLAGS) ./...

go16:
	GOROOT=$(GO16) $(GO16)/bin/go test $(TESTFLAGS) ./...

go15:
	GOROOT=$(GO15) $(GO15)/bin/go test $(TESTFLAGS) ./...
