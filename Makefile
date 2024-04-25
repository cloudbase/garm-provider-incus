SHELL := bash

.PHONY: go-test

go-test:
	go test -v ./... $(TEST_ARGS) -timeout=15m -parallel=4