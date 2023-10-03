package main

import (
	"errors"
	"net"
	"strings"
)

var ErrPrivateIpDetected = errors.New("private ip detected")

func GetPublicIp(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	parsedIp := net.ParseIP(ip)
	if parsedIp != nil && parsedIp.IsGlobalUnicast() && !parsedIp.IsPrivate() {
		return ip, nil
	}
	return "", ErrPrivateIpDetected
}
