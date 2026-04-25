## AMOS — Amos MIB Operating System
## Encodes the CGO and pkg-config environment required for Fyne.

GOPATH   ?= $(HOME)/gopath
GOROOT   ?= $(HOME)/go
DEVLIBS  ?= $(HOME)/devlibs

export PATH      := $(DEVLIBS)/usr/bin:$(PATH):$(GOROOT)/bin
export GOPATH    := $(GOPATH)
export PKG_CONFIG_PATH := \
	$(DEVLIBS)/usr/lib/x86_64-linux-gnu/pkgconfig:\
	$(DEVLIBS)/usr/share/pkgconfig:\
	/usr/lib/x86_64-linux-gnu/pkgconfig
export CGO_CFLAGS  := -I$(DEVLIBS)/usr/include
export CGO_LDFLAGS := -L$(DEVLIBS)/usr/lib/x86_64-linux-gnu -L/usr/lib/x86_64-linux-gnu
export LD_LIBRARY_PATH := $(DEVLIBS)/usr/lib/x86_64-linux-gnu:$(LD_LIBRARY_PATH)

BINARY := amos

.PHONY: all build test clean run

all: build

## build — compile the AMOS binary to ./amos
build:
	go build -o $(BINARY) ./cmd/amos/

## run — build and launch the GUI
run: build
	./$(BINARY)

## test — run all BDD and unit tests
test:
	go test ./... -timeout 120s

## clean — remove built binary
clean:
	rm -f $(BINARY)
