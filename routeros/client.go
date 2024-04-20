/*
Package routeros is a pure Go client library for accessing Mikrotik devices using the RouterOS API.
*/
package routeros

import (
	"crypto/md5" //nolint:gosec
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"

	"mikrotik-exporter/routeros/proto"
)

// Client is a RouterOS API client.
type Client struct {
	rwc     io.ReadWriteCloser
	r       proto.Reader
	w       proto.Writer
	Queue   int
	mu      sync.Mutex
	closing bool
}

// NewClient returns a new Client over rwc. Login must be called.
func NewClient(rwc io.ReadWriteCloser) (*Client, error) {
	return &Client{
		rwc: rwc,
		r:   proto.NewReader(rwc),
		w:   proto.NewWriter(rwc),
	}, nil
}

// Dial connects and logs in to a RouterOS device.
func Dial(address, username, password string) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	return newClientAndLogin(conn, username, password)
}

// DialTLS connects and logs in to a RouterOS device using TLS.
func DialTLS(address, username, password string, tlsConfig *tls.Config) (*Client, error) {
	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("tls dial error: %w", err)
	}

	return newClientAndLogin(conn, username, password)
}

func newClientAndLogin(rwc io.ReadWriteCloser, username, password string) (*Client, error) {
	client, err := NewClient(rwc)
	if err != nil {
		rwc.Close()

		return nil, err
	}

	if err = client.Login(username, password); err != nil {
		client.Close()

		return nil, err
	}

	return client, nil
}

// Close closes the connection to the RouterOS device.
func (c *Client) Close() {
	c.mu.Lock()

	if c.closing {
		c.mu.Unlock()

		return
	}

	c.closing = true
	c.mu.Unlock()
	c.rwc.Close()
}

// Login runs the /login command. Dial and DialTLS call this automatically.
func (c *Client) Login(username, password string) error {
	r, err := c.Run("/login", "=name="+username, "=password="+password)
	if err != nil {
		return err
	}

	ret, ok := r.Done.Map["ret"]
	if !ok {
		// Login method post-6.43 one stage, cleartext and no challenge
		if r.Done != nil {
			return nil
		}

		return ErrLoginNoRet
	}

	// Login method pre-6.43 two stages, challenge
	b, err := hex.DecodeString(ret)
	if err != nil {
		return fmt.Errorf("RouterOS: /login: invalid ret (challenge) hex string received: %w", err)
	}

	if _, err = c.Run("/login", "=name="+username, "=response="+c.challengeResponse(b, password)); err != nil {
		return err
	}

	return nil
}

func (c *Client) challengeResponse(cha []byte, password string) string {
	h := md5.New() //nolint:gosec
	h.Write([]byte{0})
	_, _ = io.WriteString(h, password)
	h.Write(cha)

	return fmt.Sprintf("00%x", h.Sum(nil))
}
