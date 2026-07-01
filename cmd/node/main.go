// Command node runs a minimal Warren relay: it listens for camouflaged
// connections (internal/network/transport), echoes application data back to
// authenticated Warren clients, and transparently splices anything else to a
// real fallback site. This is the L0+L1 proof of concept — no marketplace,
// payment, or ZK auth wired in yet.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/croc100/warren/internal/network/transport"
)

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:8443", "address to accept connections on")
	fallbackAddr := flag.String("fallback-addr", "", "real host:port to splice non-Warren connections to (required)")
	fallbackSNI := flag.String("fallback-sni", "", "hostname the disguised ClientHello claims to be (required)")
	pskFile := flag.String("psk-file", "", "path to a file containing the pre-shared key (32 raw bytes); if empty, reads WARREN_PSK_HEX env var")
	flag.Parse()

	if *fallbackAddr == "" || *fallbackSNI == "" {
		fmt.Fprintln(os.Stderr, "usage: node -fallback-addr=host:port -fallback-sni=example.com [-psk-file=path]")
		os.Exit(2)
	}

	psk, err := loadPSK(*pskFile)
	if err != nil {
		log.Fatalf("node: %v", err)
	}

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("node: listen %s: %v", *listenAddr, err)
	}
	log.Printf("node: listening on %s, camouflaged as %s, falling back to %s", *listenAddr, *fallbackSNI, *fallbackAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := transport.Config{PSK: psk, FallbackSNI: *fallbackSNI, FallbackAddr: *fallbackAddr}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	err = transport.Serve(ctx, ln, cfg, func(conn net.Conn) {
		defer conn.Close()
		log.Println("node: authenticated Warren client connected, echoing")
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if _, werr := conn.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	})
	if err != nil {
		log.Fatalf("node: serve: %v", err)
	}
}

func loadPSK(path string) ([]byte, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read psk file: %w", err)
		}
		if len(data) < 32 {
			return nil, fmt.Errorf("psk file must contain at least 32 bytes, got %d", len(data))
		}
		return data[:32], nil
	}
	env := os.Getenv("WARREN_PSK_HEX")
	if env == "" {
		return nil, fmt.Errorf("no PSK provided: pass -psk-file or set WARREN_PSK_HEX (64 hex chars)")
	}
	psk, err := hex.DecodeString(env)
	if err != nil {
		return nil, fmt.Errorf("WARREN_PSK_HEX: %w", err)
	}
	if len(psk) != 32 {
		return nil, fmt.Errorf("WARREN_PSK_HEX must decode to 32 bytes, got %d", len(psk))
	}
	return psk, nil
}
