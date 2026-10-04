PKG := ./...

# Tools live in their own modfile so their dependencies never become
# requirements of the library.
STATICCHECK := go tool -modfile=tools/go.mod staticcheck

.PHONY: check fmt-check tidy-check cgo-check vet lint test fmt tidy

check: fmt-check tidy-check cgo-check vet lint test

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then echo "not gofmt-formatted (run make fmt):"; echo "$$files"; exit 1; fi

tidy-check:
	go mod tidy -diff

# CGO_ENABLED=0 go build is not enough: drivers like mattn/go-sqlite3 compile
# to a stub without cgo and fail only at run time.
cgo-check:
	@pkgs="$$(CGO_ENABLED=1 go list -deps -f '{{if and (not .Standard) .CgoFiles}}{{.ImportPath}}{{end}}' $(PKG))"; \
	if [ -n "$$pkgs" ]; then echo "packages using cgo:"; echo "$$pkgs"; exit 1; fi

vet:
	go vet $(PKG)

lint:
	$(STATICCHECK) $(PKG)

test:
	go test -race $(PKG)

fmt:
	gofmt -w .

tidy:
	go mod tidy
