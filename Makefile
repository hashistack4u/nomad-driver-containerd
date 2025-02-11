BINARY ?= containerd-driver
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
	GOOS=windows $(GOLANG) build -o $(BINARY).exe .

.PHONY: test
test:
	./tests/run_tests.sh
