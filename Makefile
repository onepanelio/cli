release-windows-64:
	env GOOS=windows GOARCH=amd64 go build -o onepanel_cli_x64.exe main.go

release-mac-os-64:
	env GOOS=darwin GOARCH=amd64 go build -o onepanel_cli_mac_64 main.go

release-linux-64:
	env GOOS=linux GOARCH=amd64 go build -o onepanel_cli_linux_64 main.go

all: release-windows-64 release-mac-os-64 release-linux-64