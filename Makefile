help:
	@echo "Please use \`make <target>' where <target> is one of"
	@echo "  test                    to run all the tests"
	@echo "  build                   to build the library and jp executable"
	@echo "  generate                to run codegen"


generate:
	go generate ./...

build:
	go build ./...
	rm -f cmd/jp/jp && cd cmd/jp/ && go build ./...
	mv cmd/jp/jp .

test:
	go test -v ./...

check:
	cd jmespath
	go vet ./...
	golint ./... | fgrep -v "jmespath/astnodetype_string.go" | fgrep -v "jmespath/toktype_string.go"

htmlc:
	cd jmespath && go test -coverprofile="/tmp/jpcov" ./... && go tool cover -html="/tmp/jpcov" && unlink /tmp/jpcov
