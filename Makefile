BIN_NAME := wtf
PREFIX   ?= /usr/local
BIN_DIR  ?= $(PREFIX)/bin

.PHONY: all build run install uninstall clean

all: build
build:
	go build -o $(BIN_NAME) .

run: build
	./$(BIN_NAME)

install: build
	install -Dm 755 $(BIN_NAME) $(DESTDIR)$(BIN_DIR)/$(BIN_NAME)

uninstall:
	rm -f $(DESTDIR)$(BIN_DIR)/$(BIN_NAME)

clean:
	rm -f $(BIN_NAME)
