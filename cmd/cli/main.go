// Command cli is a minimal Warren client: it resolves a bridge via the L0
// bootstrap package, dials it through the L1 camouflage transport, and
// exercises a simple echo round-trip to prove the connection is genuinely
// authenticated and encrypted end-to-end. This is the demo/PoC client, not
// the eventual VDI-tunneling client.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/croc100/warren/internal/discovery/bootstrap"
	"github.com/croc100/warren/internal/network/transport"
)

func main() {
	bridgeFile := flag.String("bridge-file", "", "path to a locally-distributed bridge list (out-of-band channel)")
	fallbackSNI := flag.String("fallback-sni", "", "override: hostname to present in the disguised ClientHello (required if not using -bridge-file)")
	addr := flag.String("addr", "", "override: bridge host:port to dial directly (required if not using -bridge-file)")
	message := flag.String("message", "hello from a censored network", "test message to echo through the bridge")
	flag.Parse()

	psk, err := loadPSK()
	if err != nil {
		log.Fatalf("cli: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	bridge, err := resolveBridge(ctx, *bridgeFile, *addr, *fallbackSNI)
	if err != nil {
		log.Fatalf("cli: %v", err)
	}
	log.Printf("cli: dialing bridge %s (camouflaged as %s)", bridge.Addr, bridge.FallbackSNI)

	conn, err := transport.Dial(ctx, bridge.Addr, transport.Config{PSK: psk, FallbackSNI: bridge.FallbackSNI})
	if err != nil {
		log.Fatalf("cli: dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(*message)); err != nil {
		log.Fatalf("cli: write: %v", err)
	}
	buf := make([]byte, len(*message))
	r := bufio.NewReader(conn)
	if _, err := r.Read(buf); err != nil {
		log.Fatalf("cli: read: %v", err)
	}
	fmt.Printf("echoed back: %s\n", buf)
}

func resolveBridge(ctx context.Context, bridgeFile, addr, sni string) (bootstrap.Bridge, error) {
	if addr != "" && sni != "" {
		return bootstrap.Bridge{Addr: addr, FallbackSNI: sni}, nil
	}
	if bridgeFile == "" {
		return bootstrap.Bridge{}, fmt.Errorf("must pass either -bridge-file or both -addr and -fallback-sni")
	}
	m := bootstrap.Multi{Resolvers: []bootstrap.Resolver{
		bootstrap.FileResolver{Path: bridgeFile},
	}}
	bridges, results, err := m.Resolve(ctx)
	if err != nil {
		for _, r := range results {
			log.Printf("cli: discovery channel %s failed: %v", r.Channel, r.Err)
		}
		return bootstrap.Bridge{}, err
	}
	return bridges[0], nil
}

func loadPSK() ([]byte, error) {
	env := os.Getenv("WARREN_PSK_HEX")
	if env == "" {
		return nil, fmt.Errorf("no PSK provided: set WARREN_PSK_HEX (64 hex chars)")
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
