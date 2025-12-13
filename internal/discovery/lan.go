package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// LANConfig controls LAN discovery behavior.
type LANConfig struct {
	Port    int
	Timeout time.Duration
}

const (
	DefaultLANPort    = 42042
	DefaultLANTimeout = 1 * time.Second
)

// DefaultLANConfig returns the default settings for LAN discovery.
func DefaultLANConfig() LANConfig {
	return LANConfig{
		Port:    DefaultLANPort,
		Timeout: DefaultLANTimeout,
	}
}

// lanMessage is the discovery message format.
type lanMessage struct {
	Type   string `json:"type"`   // "ping" or "pong"
	Name   string `json:"name"`   // display name (optional)
	Listen string `json:"listen"` // TCP listen address, e.g. ":3001" or "192.168.1.10:3001"
}

// StartLANResponder listens for LAN discovery pings and replies with a pong
// containing this node's listen address. It runs until the provided channel is closed.
func StartLANResponder(stop <-chan struct{}, cfg LANConfig, listenAddr, name string) error {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var ctrlErr error
			if network == "udp4" || network == "udp" {
				ctrlErr = c.Control(func(fd uintptr) {
					// Allow multiple sockets to bind the same addr:port.
					_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
					// SO_REUSEPORT is not available everywhere, but it's fine if it fails.
					_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
				})
			}
			return ctrlErr
		},
	}

	conn, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("lan responder listen: %w", err)
	}

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		conn.Close()
		return fmt.Errorf("lan responder: not a UDPConn")
	}

	go func() {
		defer udpConn.Close()

		buf := make([]byte, 1024)

		for {
			select {
			case <-stop:
				return
			default:
			}

			_ = udpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

			n, addr, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					continue
				}
				continue
			}

			var msg lanMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}
			if msg.Type != "ping" {
				continue
			}

			resp := lanMessage{
				Type:   "pong",
				Name:   name,
				Listen: listenPortOnly(listenAddr),
			}
			data, _ := json.Marshal(resp)
			_, _ = udpConn.WriteToUDP(data, addr)
		}
	}()

	return nil
}

// DiscoverLANPeers broadcasts a ping on the LAN and returns any listen
// addresses reported by peers that respond within cfg.Timeout.
//
// It does NOT connect itself; the caller can decide what to do with the list.
func DiscoverLANPeers(cfg LANConfig, listenAddr, name string) ([]string, error) {
	laddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: 0,
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("lan discover listen: %w", err)
	}
	defer conn.Close()

	ping := lanMessage{
		Type:   "ping",
		Name:   name,
		Listen: listenAddr,
	}
	data, _ := json.Marshal(ping)

	targets := interfaceBroadcastAddrs(cfg.Port)
	if len(targets) == 0 {
		// fall back to limited broadcast
		targets = append(targets, &net.UDPAddr{IP: net.IPv4bcast, Port: cfg.Port})
	}
	for _, dst := range targets {
		_, err = conn.WriteToUDP(data, dst)
	}
	if err != nil {
		var opErr *net.OpError
		if errors.As(err, &opErr) && errors.Is(opErr.Err, syscall.EADDRNOTAVAIL) {
		} else {
			return nil, fmt.Errorf("lan discover broadcast: %w", err)
		}
	}

	loop := &net.UDPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: cfg.Port,
	}
	_, _ = conn.WriteToUDP(data, loop)

	// Collect replies for up to Timeout.
	if err := conn.SetReadDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return nil, fmt.Errorf("lan discover set deadline: %w", err)
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 4)
	buf := make([]byte, 1024)

	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				break
			}
			break
		}

		var msg lanMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}
		if msg.Type != "pong" {
			continue
		}
		full := normalizeListenFromPong(from, msg.Listen)
		if full == "" || full == listenAddr || msg.Listen == listenAddr {
			continue
		}
		if _, exists := seen[msg.Listen]; exists {
			continue
		}
		seen[msg.Listen] = struct{}{}
		out = append(out, full)
	}

	return out, nil
}

func interfaceBroadcastAddrs(port int) []*net.UDPAddr {
	out := make([]*net.UDPAddr, 0, 8)

	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}

	for _, it := range ifaces {
		// skip down interfaces
		if it.Flags&net.FlagUp == 0 {
			continue
		}
		// skip point-to-point/tunnel-ish
		if it.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		addrs, err := it.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP == nil {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}

			// compute broadcast = ip | ^mask
			mask := ipnet.Mask
			if len(mask) != 4 {
				continue
			}
			b := net.IPv4(
				ip4[0]|^mask[0],
				ip4[1]|^mask[1],
				ip4[2]|^mask[2],
				ip4[3]|^mask[3],
			)
			out = append(out, &net.UDPAddr{IP: b, Port: port})
		}
	}
	return out
}
