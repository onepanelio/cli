ifndef version
	$(error CLI version is undefined)
endif
ifndef manifests-version-tag
	$(error manifests version tag is undefined)
endif
ifndef core-version-tag
	$(error core version tag is undefined)
endif
ifndef core-ui-version-tag
	$(error core-ui version tag is undefined)
endif

	ldflags := "\
		-X github.com/onepanelio/cli/config.CLIVersion=$(version)\
		-X github.com/onepanelio/cli/config.ManifestsRepositoryTag=$(manifests-version-tag)\
		-X github.com/onepanelio/cli/config.CoreImageTag=$(core-version-tag)\
		-X github.com/onepanelio/cli/config.CoreUIImageTag=$(core-ui-version-tag)"

build-linux-amd64:
	env GOOS=linux GOARCH=amd64 go build \
			-o opctl-linux-amd64 \
			-ldflags $(ldflags) \
			main.go

build-macos-amd64:
	env GOOS=darwin GOARCH=amd64 go build \
			-o opctl-macos-amd64 \
			-ldflags $(ldflags) \
			main.go

build-windows-amd64:
	env GOOS=windows GOARCH=amd64 go build \
			-o opctl-windows-amd64.exe \
			-ldflags $(ldflags) \
			main.go

all-internal: build-linux-amd64 build-macos-amd64 build-windows-amd64

all:
	docker run --rm \
 	-e version=$(version) \
 	-e manifests-version-tag=$(manifests-version-tag) \
 	-e core-version-tag=$(core-version-tag) \
 	-e core-ui-version-tag=$(core-ui-version-tag) \
 	-v "$(PWD)":/usr/src/myapp -w /usr/src/myapp golang:1.15 \
 	make all-internal
