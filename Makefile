build-windows-amd64:
	env GOOS=windows GOARCH=amd64 go build -o opctl-windows-amd64.exe main.go

build-darwin-amd64:
	env GOOS=darwin GOARCH=amd64 go build -o opctl-darwin-amd64 main.go

build-linux-amd64:
	env GOOS=linux GOARCH=amd64 go build -o opctl-linux-amd64 main.go

all: build-windows-amd64 build-darwin-amd64 build-linux-amd64