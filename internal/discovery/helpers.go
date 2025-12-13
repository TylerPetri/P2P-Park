package discovery

import (
	"net"
	"strings"
)

func listenPortOnly(listenAddr string) string {
	// returns ":12345"
	_, port, err := net.SplitHostPort(listenAddr)
	if err == nil && port != "" {
		return ":" + port
	}
	// try raw ":1234"
	if strings.HasPrefix(listenAddr, ":") {
		return listenAddr
	}
	return listenAddr
}

func normalizeListenFromPong(sender *net.UDPAddr, listen string) string {
	// if listen is ":port", join with sender IP
	if strings.HasPrefix(listen, ":") && sender != nil && sender.IP != nil {
		return net.JoinHostPort(sender.IP.String(), strings.TrimPrefix(listen, ":"))
	}
	return listen
}
