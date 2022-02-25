BINARY ?= npd
BINDIR ?= $(DESTDIR)/usr/local/bin
ifndef $(GOLANG)
    GOLANG=$(shell which go)
    export GOLANG
endif

export GO111MODULE=on
export NOMAD_ADDR=http://localhost:4646
export NOMAD_E2E=1

default: build

.PHONY: clean
clean:
	rm -f $(BINARY)

.PHONY: build
build:
	$(GOLANG) build -o $(BINARY) .

.PHONY: install
install:
	$(GOLANG) build -o $(BINARY) .
	install -m 755 $(BINARY) $(BINDIR)/$(BINARY)

.PHONY: test
test:
	$(GOLANG) test -count=1 -v ./...
