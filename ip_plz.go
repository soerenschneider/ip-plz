package main

import (
	"log"
	"net"
	"net/http"
	"strings"
)

type IpPlz struct {
	headers []string
}

func NewIpPlz(trustedHeaders []string) *IpPlz {
	return &IpPlz{
		headers: trustedHeaders,
	}
}

func (b *IpPlz) getIp(req *http.Request) string {
	for _, h := range b.headers {
		for _, ip := range strings.Split(req.Header.Get(h), ",") {
			ip = strings.TrimSpace(ip)
			parsedIp := net.ParseIP(ip)
			if parsedIp != nil && parsedIp.IsGlobalUnicast() && !parsedIp.IsPrivate() {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil {
		return host
	}

	return req.RemoteAddr
}

func (b *IpPlz) detectIp(w http.ResponseWriter, req *http.Request) {
	requestsTotal.Inc()
	requestsTimestamp.SetToCurrentTime()
	pubIp := b.getIp(req)
	_, err := w.Write([]byte(pubIp))
	if err != nil {
		log.Printf("detectIp: error writing to writer: %v", err)
	}
}

func (b *IpPlz) healthcheckHandler(w http.ResponseWriter, req *http.Request) {
	_, err := w.Write([]byte("pong"))
	if err != nil {
		log.Printf("healthcheckHandler: error writing to writer: %v", err)
	}
}
