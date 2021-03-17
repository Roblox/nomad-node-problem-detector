BINARY ?= npd
BINDIR ?= $(DESTDIR)/usr/local/bin
ifndef $(GOLANG)
    GOLANG=$(shell which go)
    export GOLANG
endif

export GO111MODULE=on
export GOOS=linux

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
