VERSION=$(shell git log -1 --pretty=tformat:%ad-%h --date=format:'%d.%m.%y')

.PHONY: test
test:
	@go test -race -cover $(shell go list ./...)
