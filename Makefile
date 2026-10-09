.PHONY: build install test vet lint reference compare wctype

build:
	go build -o softmagic ./cmd/softmagic

install:
	go install ./cmd/softmagic

test:
	go test -race -count=1 ./...

# Builds file(1) 5.48 from its verified tarball into .reference/.
reference:
	./scripts/reference.sh

# Runs softmagic and the reference side by side over the corpus and DIRS
# (colon-separated, walked recursively), in every output mode.
compare: reference
	SOFTMAGIC_REFERENCE_DIRS="$(DIRS)" go test -count=1 -timeout 60m -run TestReference -v ./cmd/softmagic

# Regenerates the iswprint and wcwidth table from this host's glibc.
wctype:
	cc -O2 -o .wctype scripts/wctype.c
	./.wctype main > cmd/softmagic/wctype.go
	rm -f .wctype

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
