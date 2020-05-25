build-linux-amd64:
	env GOOS=linux GOARCH=amd64 go build -o opctl-linux-amd64 main.go

build-macos-amd64:
	env GOOS=darwin GOARCH=amd64 go build -o opctl-macos-amd64 main.go

build-windows-amd64:
	env GOOS=windows GOARCH=amd64 go build -o opctl-windows-amd64.exe main.go

all: build-linux-amd64 build-macos-amd64 build-windows-amd64