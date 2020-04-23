package util

import (
	"net"
	"strings"
)

func IsIpv4(host string) bool {
	return net.ParseIP(strings.Trim(host, "'")) != nil
}
