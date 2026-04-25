// Package snmp wraps gosnmp to provide SNMP v1/v2c/v3 operations.
// All public functions are safe to call from multiple goroutines (T-07).
package snmp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/gosnmp/gosnmp"
)

// Version represents SNMP protocol version.
type Version int

const (
	Version1  Version = 1
	Version2c Version = 2
	Version3  Version = 3
)

// V3AuthProto selects authentication algorithm for SNMPv3.
type V3AuthProto int

const (
	AuthNone V3AuthProto = iota
	AuthMD5
	AuthSHA
)

// V3PrivProto selects encryption algorithm for SNMPv3.
type V3PrivProto int

const (
	PrivNone V3PrivProto = iota
	PrivDES
	PrivAES
)

// Target describes an SNMP endpoint.
type Target struct {
	Host      string
	Port      uint16
	Version   Version
	Community string // v1/v2c
	// v3 fields
	Username  string
	AuthProto V3AuthProto
	AuthPass  string
	PrivProto V3PrivProto
	PrivPass  string
	Timeout   time.Duration
	Retries   int
}

// Result holds one SNMP variable binding.
type Result struct {
	OID   string
	Type  string
	Value interface{}
	Error string // non-empty if this binding had a per-variable error
}

// Client holds a pooled gosnmp.GoSNMP connection per target.
// All methods are goroutine-safe (T-07).
type Client struct {
	mu     sync.Mutex
	params *g.GoSNMP
	target Target
}

// pool holds one Client per target key.
var (
	poolMu sync.Mutex
	pool   = map[string]*Client{}
)

// clientKey produces a canonical key for the target.
func clientKey(t Target) string {
	return fmt.Sprintf("%s:%d:v%d:%s:%s", t.Host, t.Port, t.Version, t.Community, t.Username)
}

// GetClient returns a pooled Client for the given target, creating one if needed.
func GetClient(t Target) *Client {
	key := clientKey(t)
	poolMu.Lock()
	defer poolMu.Unlock()
	if c, ok := pool[key]; ok {
		return c
	}
	c := &Client{target: t, params: buildParams(t)}
	pool[key] = c
	return c
}

// RemoveClient evicts a cached client (e.g. after a connection change).
func RemoveClient(t Target) {
	key := clientKey(t)
	poolMu.Lock()
	defer poolMu.Unlock()
	if c, ok := pool[key]; ok {
		_ = c.params.Conn.Close()
		delete(pool, key)
	}
}

func buildParams(t Target) *g.GoSNMP {
	timeout := t.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	retries := t.Retries
	if retries == 0 {
		retries = 1
	}

	params := &g.GoSNMP{
		Target:    t.Host,
		Port:      t.Port,
		Timeout:   timeout,
		Retries:   retries,
		MaxOids:   g.MaxOids,
	}

	switch t.Version {
	case Version1:
		params.Version = g.Version1
		params.Community = t.Community
	case Version2c:
		params.Version = g.Version2c
		params.Community = t.Community
	case Version3:
		params.Version = g.Version3
		params.SecurityModel = g.UserSecurityModel
		sp := &g.UsmSecurityParameters{
			UserName:               t.Username,
			AuthoritativeEngineID:  "",
		}
		switch t.AuthProto {
		case AuthMD5:
			params.MsgFlags = g.AuthNoPriv
			sp.AuthenticationProtocol = g.MD5
			sp.AuthenticationPassphrase = t.AuthPass
		case AuthSHA:
			params.MsgFlags = g.AuthNoPriv
			sp.AuthenticationProtocol = g.SHA
			sp.AuthenticationPassphrase = t.AuthPass
		}
		switch t.PrivProto {
		case PrivDES:
			params.MsgFlags = g.AuthPriv
			sp.PrivacyProtocol = g.DES
			sp.PrivacyPassphrase = t.PrivPass
		case PrivAES:
			params.MsgFlags = g.AuthPriv
			sp.PrivacyProtocol = g.AES
			sp.PrivacyPassphrase = t.PrivPass
		}
		if t.AuthProto == AuthNone && t.PrivProto == PrivNone {
			params.MsgFlags = g.NoAuthNoPriv
		}
		params.SecurityParameters = sp
	}
	return params
}

// connect ensures the gosnmp connection is open.  Must be called under c.mu.
func (c *Client) connect() error {
	if c.params.Conn != nil {
		return nil
	}
	return c.params.Connect()
}

// Get fetches one or more OIDs.  Runs within ctx deadline (T-01).
func (c *Client) Get(ctx context.Context, oids []string) ([]Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	done := make(chan struct{ res []Result; err error }, 1)
	go func() {
		pkt, err := c.params.Get(oids)
		if err != nil {
			done <- struct{ res []Result; err error }{nil, err}
			return
		}
		done <- struct{ res []Result; err error }{convertPDU(pkt), nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.res, r.err
	}
}

// GetNext fetches the next OID after each supplied OID.
func (c *Client) GetNext(ctx context.Context, oids []string) ([]Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	done := make(chan struct{ res []Result; err error }, 1)
	go func() {
		pkt, err := c.params.GetNext(oids)
		if err != nil {
			done <- struct{ res []Result; err error }{nil, err}
			return
		}
		done <- struct{ res []Result; err error }{convertPDU(pkt), nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.res, r.err
	}
}

// GetBulk retrieves up to maxReps varbinds for each non-repeater OID (v2c/v3 only).
// maxReps is bounded by the caller per T-06.
func (c *Client) GetBulk(ctx context.Context, oids []string, nonRepeaters, maxReps uint32) ([]Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.target.Version == Version1 {
		return nil, fmt.Errorf("GETBULK is not supported on SNMP v1")
	}
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	done := make(chan struct{ res []Result; err error }, 1)
	go func() {
		pkt, err := c.params.GetBulk(oids, uint8(nonRepeaters), maxReps)
		if err != nil {
			done <- struct{ res []Result; err error }{nil, err}
			return
		}
		done <- struct{ res []Result; err error }{convertPDU(pkt), nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.res, r.err
	}
}

// Walk performs a subtree walk using GETNEXT (v1) or GETBULK (v2c/v3).
// Results are streamed to the returned channel; the channel is closed on completion.
// The caller can cancel via ctx (T-01, T-05).
func (c *Client) Walk(ctx context.Context, rootOID string) (<-chan Result, <-chan error) {
	out := make(chan Result, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errCh)

		c.mu.Lock()
		if err := c.connect(); err != nil {
			c.mu.Unlock()
			errCh <- fmt.Errorf("connect: %w", err)
			return
		}

		// Use WalkAll for v1 (GetNext), BulkWalkAll for v2c/v3.
		var results []g.SnmpPDU
		var err error
		if c.target.Version == Version1 {
			results, err = c.params.WalkAll(rootOID)
		} else {
			results, err = c.params.BulkWalkAll(rootOID)
		}
		c.mu.Unlock()

		if err != nil {
			errCh <- err
			return
		}

		for _, pdu := range results {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case out <- pduToResult(pdu):
			}
		}
	}()

	return out, errCh
}

// Set writes one or more OID→value pairs to the target.
func (c *Client) Set(ctx context.Context, pdus []g.SnmpPDU) ([]Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	done := make(chan struct{ res []Result; err error }, 1)
	go func() {
		pkt, err := c.params.Set(pdus)
		if err != nil {
			done <- struct{ res []Result; err error }{nil, err}
			return
		}
		done <- struct{ res []Result; err error }{convertPDU(pkt), nil}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		return r.res, r.err
	}
}

// convertPDU converts a gosnmp SnmpPacket into a slice of Results.
// PDU-level errors are surfaced in Result.Error (T-04).
func convertPDU(pkt *g.SnmpPacket) []Result {
	if pkt == nil {
		return nil
	}
	out := make([]Result, 0, len(pkt.Variables))
	for _, v := range pkt.Variables {
		out = append(out, pduToResult(v))
	}
	return out
}

func pduToResult(v g.SnmpPDU) Result {
	r := Result{
		OID:   strings.TrimPrefix(v.Name, "."),
		Type:  v.Type.String(),
		Value: v.Value,
	}
	switch v.Type {
	case g.NoSuchObject, g.NoSuchInstance, g.EndOfMibView:
		r.Error = v.Type.String()
	}
	return r
}
