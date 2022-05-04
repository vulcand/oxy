.PHONY: all

export GO111MODULE=on

default: clean checks test

test: clean
	go test -race -cover -count 1 ./...

test-verbose: clean
	go test -v -race -cover ./...

clean:
	find . -name flymake_* -delete
	rm -f cover.out

checks:
	golangci-lint run
