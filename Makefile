.PHONY: build install test vet lint

build:
	go build -o softmagic ./cmd/softmagic

install:
	go install ./cmd/softmagic

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint: vet
	gofmt -l . | (! grep .)
	go tool staticcheck ./...
	# G304 (file opened from a variable) is this program's purpose: it
	# opens the files named on its command line, uncleaned, as file(1) does.
	go tool gosec -quiet -exclude=G304 ./...
	go tool govulncheck ./...
	go tool golangci-lint run ./...
