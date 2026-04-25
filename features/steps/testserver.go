// Package steps — minimal in-process SNMP v2c UDP server for BDD tests.
// Supports GET (a0) and GETNEXT (a1) requests for a fixed OID database.
package steps

import (
	"fmt"
	"net"
	"sort"
	"sync"
)

// ---- test fixture OID database ----

var testOIDs = []string{
	"1.3.6.1.2.1.1.1.0",
	"1.3.6.1.2.1.1.2.0",
	"1.3.6.1.2.1.1.3.0",
	"1.3.6.1.2.1.1.4.0",
	"1.3.6.1.2.1.1.5.0",
	"1.3.6.1.2.1.1.6.0",
	"1.3.6.1.2.1.1.7.0",
}

var testValues = map[string]string{
	"1.3.6.1.2.1.1.1.0": "AMOS BDD Test Agent",
	"1.3.6.1.2.1.1.2.0": "1.3.6.1.4.1.99999",
	"1.3.6.1.2.1.1.3.0": "12345",
	"1.3.6.1.2.1.1.4.0": "admin@example.com",
	"1.3.6.1.2.1.1.5.0": "test-host.local",
	"1.3.6.1.2.1.1.6.0": "Test Location",
	"1.3.6.1.2.1.1.7.0": "72",
}

func init() {
	sort.Slice(testOIDs, func(i, j int) bool { return oidLess(testOIDs[i], testOIDs[j]) })
}

// ---- server ----

// TestSNMPServer is a minimal SNMP v2c UDP GET/GETNEXT responder.
type TestSNMPServer struct {
	conn *net.UDPConn
	done chan struct{}
	wg   sync.WaitGroup
}

// NewTestSNMPServer starts the server on a random localhost port.
func NewTestSNMPServer() (*TestSNMPServer, error) {
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("test snmp server listen: %w", err)
	}
	s := &TestSNMPServer{conn: conn, done: make(chan struct{})}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Port returns the UDP port in use.
func (s *TestSNMPServer) Port() int {
	return s.conn.LocalAddr().(*net.UDPAddr).Port
}

// Close shuts down the server.
func (s *TestSNMPServer) Close() {
	close(s.done)
	s.conn.Close()
	s.wg.Wait()
}

func (s *TestSNMPServer) serve() {
	defer s.wg.Done()
	buf := make([]byte, 65535)
	for {
		select {
		case <-s.done:
			return
		default:
		}
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		go func(pkt []byte, addr *net.UDPAddr) {
			if resp := processPacket(pkt); resp != nil {
				s.conn.WriteToUDP(resp, addr)
			}
		}(pkt, addr)
	}
}

// ---- PDU processing ----

func processPacket(pkt []byte) []byte {
	// Parse outer SEQUENCE
	if len(pkt) < 2 || pkt[0] != 0x30 {
		return nil
	}
	payload, ok := berParseLen(pkt[1:])
	if !ok {
		return nil
	}

	// version INTEGER
	version, payload, ok := berParseInt(payload)
	if !ok || version != 1 {
		return nil
	}

	// community OCTET STRING
	community, payload, ok := berParseOctetString(payload)
	if !ok {
		return nil
	}

	// PDU
	if len(payload) < 2 {
		return nil
	}
	pduTag := payload[0]
	if pduTag != 0xa0 && pduTag != 0xa1 && pduTag != 0xa5 { // GET, GETNEXT, GETBULK
		return nil
	}
	pduPayload, ok := berParseLen(payload[1:])
	if !ok {
		return nil
	}

	reqID, pduPayload, ok := berParseInt(pduPayload)
	if !ok {
		return nil
	}
	// GETBULK: second field = non-repeaters, third = max-repetitions
	// GET/GETNEXT: second = error-status (0), third = error-index (0)
	_, pduPayload, _ = berParseInt(pduPayload) // non-repeaters / error-status
	maxReps, pduPayload, _ := berParseInt(pduPayload)
	if pduTag != 0xa5 || maxReps <= 0 {
		maxReps = 10
	}

	// VarBind list SEQUENCE
	if len(pduPayload) < 2 || pduPayload[0] != 0x30 {
		return nil
	}
	varList, ok := berParseLen(pduPayload[1:])
	if !ok {
		return nil
	}

	var responseOIDs []string
	var responseVals []string

	for len(varList) > 0 {
		if varList[0] != 0x30 {
			break
		}
		vbPayload, ok2 := berParseLen(varList[1:])
		if !ok2 {
			break
		}
		oid, _, ok3 := berParseOID(vbPayload)
		if !ok3 {
			break
		}

		var respOID, respVal string
		switch pduTag {
		case 0xa0: // GET
			respOID = oid
			if v, found := testValues[oid]; found {
				respVal = v
			}
			responseOIDs = append(responseOIDs, respOID)
			responseVals = append(responseVals, respVal)
		case 0xa1: // GETNEXT
			respOID, respVal = nextOID(oid)
			responseOIDs = append(responseOIDs, respOID)
			responseVals = append(responseVals, respVal)
		case 0xa5: // GETBULK: return up to maxReps OIDs starting after oid
			cur := oid
			for i := int64(0); i < maxReps; i++ {
				next, val := nextOID(cur)
				if next == "" || next == cur {
					break
				}
				responseOIDs = append(responseOIDs, next)
				responseVals = append(responseVals, val)
				cur = next
			}
		}

		// Advance past this varbind (tag + length + body)
		vbFullLen := tlvSize(varList[1:])
		varList = varList[1+vbFullLen:]
	}

	return buildGetResponse(community, reqID, responseOIDs, responseVals)
}

func nextOID(oid string) (string, string) {
	for _, k := range testOIDs {
		if oidLess(oid, k) {
			return k, testValues[k]
		}
	}
	return oid, ""
}

// ---- BER builder ----

func buildGetResponse(community string, reqID int64, oids []string, vals []string) []byte {
	var varBinds []byte
	for i, oid := range oids {
		var valBytes []byte
		if vals[i] == "" {
			valBytes = []byte{0x05, 0x00} // NULL for unknown OID
		} else {
			valBytes = berEncodeOctetString([]byte(vals[i]))
		}
		vb := berSeq(append(berEncodeOID(oid), valBytes...))
		varBinds = append(varBinds, vb...)
	}

	pduContent := berEncodeInt(reqID)
	pduContent = append(pduContent, berEncodeInt(0)...) // error-status
	pduContent = append(pduContent, berEncodeInt(0)...) // error-index
	pduContent = append(pduContent, berSeq(varBinds)...)

	// GetResponse PDU tag = 0xa2
	pdu := berContextConstructed(0xa2, pduContent)

	msg := berEncodeInt(1) // version = v2c
	msg = append(msg, berEncodeOctetString([]byte(community))...)
	msg = append(msg, pdu...)
	return berSeq(msg)
}

// ---- BER encoding ----

func berEncodeLen(n int) []byte {
	switch {
	case n < 128:
		return []byte{byte(n)}
	case n < 256:
		return []byte{0x81, byte(n)}
	default:
		return []byte{0x82, byte(n >> 8), byte(n)}
	}
}

func berSeq(body []byte) []byte {
	return append(append([]byte{0x30}, berEncodeLen(len(body))...), body...)
}

func berContextConstructed(tag byte, body []byte) []byte {
	return append(append([]byte{tag}, berEncodeLen(len(body))...), body...)
}

func berEncodeInt(v int64) []byte {
	if v == 0 {
		return []byte{0x02, 0x01, 0x00}
	}
	// Encode minimal big-endian signed integer.
	var b [8]byte
	for i := 7; i >= 0; i-- {
		b[i] = byte(v & 0xff)
		v >>= 8
	}
	start := 0
	for start < 7 && b[start] == 0 && b[start+1]&0x80 == 0 {
		start++
	}
	content := b[start:]
	return append([]byte{0x02, byte(len(content))}, content...)
}

func berEncodeOctetString(s []byte) []byte {
	return append(append([]byte{0x04}, berEncodeLen(len(s))...), s...)
}

func berEncodeOID(oid string) []byte {
	parts := splitOIDParts(oid)
	if len(parts) < 2 {
		return []byte{0x06, 0x01, 0x00}
	}
	var body []byte
	body = append(body, berBase128(parts[0]*40+parts[1])...)
	for _, p := range parts[2:] {
		body = append(body, berBase128(p)...)
	}
	return append(append([]byte{0x06}, berEncodeLen(len(body))...), body...)
}

func berBase128(v uint32) []byte {
	if v == 0 {
		return []byte{0x00}
	}
	var buf [5]byte
	i := 4
	for v > 0 {
		buf[i] = byte(v & 0x7f)
		v >>= 7
		i--
	}
	for j := i + 1; j < 4; j++ {
		buf[j] |= 0x80
	}
	return buf[i+1:]
}

// ---- BER decoding ----

// berParseLen consumes the length octets and returns the body slice.
func berParseLen(data []byte) (body []byte, ok bool) {
	if len(data) < 1 {
		return nil, false
	}
	if data[0] < 0x80 {
		l := int(data[0])
		if len(data) < 1+l {
			return nil, false
		}
		return data[1 : 1+l], true
	}
	n := int(data[0] & 0x7f)
	if n == 0 || len(data) < 1+n {
		return nil, false
	}
	var l int
	for i := 0; i < n; i++ {
		l = l<<8 | int(data[1+i])
	}
	if len(data) < 1+n+l {
		return nil, false
	}
	return data[1+n : 1+n+l], true
}

func berParseInt(data []byte) (v int64, rest []byte, ok bool) {
	if len(data) < 2 || data[0] != 0x02 {
		return 0, data, false
	}
	l := int(data[1])
	if len(data) < 2+l {
		return 0, data, false
	}
	for _, b := range data[2 : 2+l] {
		v = v<<8 | int64(b)
	}
	return v, data[2+l:], true
}

func berParseOctetString(data []byte) (s string, rest []byte, ok bool) {
	if len(data) < 2 || data[0] != 0x04 {
		return "", data, false
	}
	l := int(data[1])
	if len(data) < 2+l {
		return "", data, false
	}
	return string(data[2 : 2+l]), data[2+l:], true
}

func berParseOID(data []byte) (oid string, rest []byte, ok bool) {
	if len(data) < 2 || data[0] != 0x06 {
		return "", data, false
	}
	l := int(data[1])
	if len(data) < 2+l {
		return "", data, false
	}
	body := data[2 : 2+l]
	if len(body) == 0 {
		return "0.0", data[2+l:], true
	}
	first := uint32(body[0])
	parts := []uint32{first / 40, first % 40}
	i := 1
	for i < len(body) {
		var v uint32
		for i < len(body) {
			b := body[i]
			i++
			v = v<<7 | uint32(b&0x7f)
			if b&0x80 == 0 {
				break
			}
		}
		parts = append(parts, v)
	}
	s := ""
	for j, p := range parts {
		if j > 0 {
			s += "."
		}
		s += fmt.Sprintf("%d", p)
	}
	return s, data[2+l:], true
}

// tlvSize returns the total byte count of the TLV whose length field starts at data[0].
func tlvSize(data []byte) int {
	if len(data) < 1 {
		return 0
	}
	if data[0] < 0x80 {
		return 1 + int(data[0])
	}
	n := int(data[0] & 0x7f)
	if len(data) < 1+n {
		return 0
	}
	var l int
	for i := 0; i < n; i++ {
		l = l<<8 | int(data[1+i])
	}
	return 1 + n + l
}

// ---- OID utilities ----

func splitOIDParts(oid string) []uint32 {
	var parts []uint32
	var cur uint32
	hasCur := false
	for _, c := range oid {
		switch {
		case c == '.':
			if hasCur {
				parts = append(parts, cur)
			}
			cur, hasCur = 0, false
		case c >= '0' && c <= '9':
			cur = cur*10 + uint32(c-'0')
			hasCur = true
		}
	}
	if hasCur {
		parts = append(parts, cur)
	}
	return parts
}

func oidLess(a, b string) bool {
	pa, pb := splitOIDParts(a), splitOIDParts(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if pa[i] < pb[i] {
			return true
		}
		if pa[i] > pb[i] {
			return false
		}
	}
	return len(pa) < len(pb)
}
