package ui

import (
	"fmt"
	"net"

	g "github.com/gosnmp/gosnmp"
)

// parseCIDROrIP parses either a CIDR block or a bare IP.
func parseCIDROrIP(s string) (net.IP, *net.IPNet, error) {
	if ip := net.ParseIP(s); ip != nil {
		mask := net.CIDRMask(32, 32)
		return ip, &net.IPNet{IP: ip, Mask: mask}, nil
	}
	ip, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid IP or CIDR: %s", s)
	}
	return ip, ipNet, nil
}

// toGoPDUs converts snmpSetPDU structs into gosnmp SnmpPDU values.
func toGoPDUs(pdus []snmpSetPDU) []g.SnmpPDU {
	out := make([]g.SnmpPDU, 0, len(pdus))
	for _, p := range pdus {
		out = append(out, g.SnmpPDU{
			Name:  p.OID,
			Type:  stringToAsn1BER(p.Type),
			Value: p.Value,
		})
	}
	return out
}

// stringToAsn1BER maps a human-readable type name to gosnmp Asn1BER.
func stringToAsn1BER(typeName string) g.Asn1BER {
	switch typeName {
	case "Integer":
		return g.Integer
	case "OctetString":
		return g.OctetString
	case "OID":
		return g.ObjectIdentifier
	case "IPAddress":
		return g.IPAddress
	case "Counter32":
		return g.Counter32
	case "Gauge32":
		return g.Gauge32
	case "TimeTicks":
		return g.TimeTicks
	case "Counter64":
		return g.Counter64
	default:
		return g.OctetString
	}
}
