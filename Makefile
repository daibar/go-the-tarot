BINARY := gtarot
PREFIX ?= $(HOME)/.local
BINDIR := $(PREFIX)/bin

.PHONY: build install uninstall test clean

build:
	go build -o $(BINARY) .

install: build
	install -d $(BINDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)

uninstall:
	rm -f $(BINDIR)/$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
