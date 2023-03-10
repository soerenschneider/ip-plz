BUILD_DIR = builds
MODULE = github.com/soerenschneider/ip-plz
BINARY_NAME = ip-plz
CHECKSUM_FILE = checksum.sha256
SIGNATURE_KEYFILE = ~/.signify/github.sec

tests:
	go test ./... -covermode=count -coverprofile=coverage.out
	go tool cover -html=coverage.out -o=coverage.html
	go tool cover -func=coverage.out -o=coverage.out

clean:
	git diff --quiet || { echo 'Dirty work tree' ; false; }
	rm -rf ./$(BUILD_DIR)

build: version-info
	CGO_ENABLED=0 go build -ldflags="-X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BINARY_NAME) main.go

release: clean version-info cross-build
	cd $(BUILD_DIR) && sha256sum * > $(CHECKSUM_FILE) && cd -

signed-release: release
	pass keys/signify/github | signify -S -s $(SIGNATURE_KEYFILE) -m $(BUILD_DIR)/$(CHECKSUM_FILE)
	gh-upload-assets -o soerenschneider -r $(BINARY_NAME) -f ~/.gh-token builds

cross-build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0       go build -ldflags="-X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64   main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0       go build -ldflags="-X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-aarch64 main.go

version-info:
	$(eval VERSION := $(shell git describe --tags --abbrev=0 || echo "dev"))
	$(eval COMMIT_HASH := $(shell git rev-parse HEAD))

fmt:
	find . -iname "*.go" -exec go fmt {} \; 

pre-commit-init:
	pre-commit install
	pre-commit install --hook-type commit-msg

pre-commit-update:
	pre-commit autoupdate
