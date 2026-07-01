package transport

import (
	"encoding/binary"
	"errors"
	"io"
	"net"

	"golang.org/x/crypto/chacha20poly1305"
)

const maxFrameLen = 1 << 16 // 64KiB, keeps frames well under typical MTU-driven fragmentation

var errFrameTooLarge = errors.New("transport: frame exceeds maxFrameLen")

// aeadConn wraps a net.Conn with ChaCha20-Poly1305 framing keyed by a session
// key both peers derived independently via deriveSessionKey. Client and
// server use disjoint nonce spaces (odd/even counters) so the same key can
// safely encrypt both directions without a nonce collision.
type aeadConn struct {
	net.Conn
	aead interface {
		Seal(dst, nonce, plaintext, additionalData []byte) []byte
		Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
		NonceSize() int
		Overhead() int
	}
	sendCounter uint64
	recvCounter uint64
	sendOdd     bool
	readBuf     []byte
}

func newAEADConn(conn net.Conn, key []byte, isClient bool) (*aeadConn, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	return &aeadConn{Conn: conn, aead: aead, sendOdd: isClient}, nil
}

func (c *aeadConn) nonce(counter uint64, odd bool) []byte {
	n := make([]byte, c.aead.NonceSize())
	if odd {
		counter |= 1 << 63
	}
	binary.BigEndian.PutUint64(n[c.aead.NonceSize()-8:], counter)
	return n
}

func (c *aeadConn) Write(p []byte) (int, error) {
	total := 0
	for len(p) > 0 {
		chunk := p
		if len(chunk) > maxFrameLen {
			chunk = chunk[:maxFrameLen]
		}
		sealed := c.aead.Seal(nil, c.nonce(c.sendCounter, c.sendOdd), chunk, nil)
		c.sendCounter++

		var lenPrefix [4]byte
		binary.BigEndian.PutUint32(lenPrefix[:], uint32(len(sealed)))
		if _, err := c.Conn.Write(lenPrefix[:]); err != nil {
			return total, err
		}
		if _, err := c.Conn.Write(sealed); err != nil {
			return total, err
		}
		total += len(chunk)
		p = p[len(chunk):]
	}
	return total, nil
}

func (c *aeadConn) Read(p []byte) (int, error) {
	if len(c.readBuf) == 0 {
		var lenPrefix [4]byte
		if _, err := io.ReadFull(c.Conn, lenPrefix[:]); err != nil {
			return 0, err
		}
		n := binary.BigEndian.Uint32(lenPrefix[:])
		if n > maxFrameLen+uint32(c.aead.Overhead()) {
			return 0, errFrameTooLarge
		}
		sealed := make([]byte, n)
		if _, err := io.ReadFull(c.Conn, sealed); err != nil {
			return 0, err
		}
		plain, err := c.aead.Open(nil, c.nonce(c.recvCounter, !c.sendOdd), sealed, nil)
		if err != nil {
			return 0, err
		}
		c.recvCounter++
		c.readBuf = plain
	}
	n := copy(p, c.readBuf)
	c.readBuf = c.readBuf[n:]
	return n, nil
}
