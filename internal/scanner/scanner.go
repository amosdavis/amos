// Package scanner discovers SNMP-responsive hosts on a subnet.
package scanner

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	g "github.com/gosnmp/gosnmp"
)

// Found describes a discovered SNMP agent.
type Found struct {
	IP        string
	SysDescr  string
	Community string
	Version   string
}

// Scanner probes every host in a CIDR block for SNMP on UDP/161.
// It honours context cancellation (T-05).
type Scanner struct {
	Communities []string
	Timeout     time.Duration
	Workers     int
	Port        uint16 // defaults to 161
}

// New returns a Scanner with sensible defaults.
func New() *Scanner {
	return &Scanner{
		Communities: []string{"public", "private"},
		Timeout:     250 * time.Millisecond,
		Workers:     64,
		Port:        161,
	}
}

// Scan probes all hosts in the given CIDR notation.
// Results are streamed to the returned channel; it is closed when the scan finishes.
// progress receives 0..100 as a percentage (approximate).
func (s *Scanner) Scan(ctx context.Context, cidr string, progress chan<- int) (<-chan Found, <-chan error) {
	out := make(chan Found, 32)
	errCh := make(chan error, 1)

	ips, err := hostsInCIDR(cidr)
	if err != nil {
		go func() {
			errCh <- fmt.Errorf("invalid CIDR %q: %w", cidr, err) // F-12
			close(out)
			close(errCh)
		}()
		return out, errCh
	}

	go func() {
		defer close(out)
		defer close(errCh)
		s.run(ctx, ips, out, progress)
	}()

	return out, errCh
}

func (s *Scanner) run(ctx context.Context, ips []string, out chan<- Found, progress chan<- int) {
	total := len(ips)
	if total == 0 {
		return
	}

	sem := make(chan struct{}, s.Workers)
	var wg sync.WaitGroup
	var done int
	var mu sync.Mutex

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			defer func() { <-sem }()

			found := s.probeHost(ctx, host)
			mu.Lock()
			done++
			pct := done * 100 / total
			mu.Unlock()

			if progress != nil {
				select {
				case progress <- pct:
				default:
				}
			}

			if found != nil {
				select {
				case out <- *found:
				case <-ctx.Done():
				}
			}
		}(ip)
	}
	wg.Wait()
}

// probeHost attempts SNMP GET on sysDescr.0 using each community string.
// Returns nil if the host does not respond.
func (s *Scanner) probeHost(ctx context.Context, host string) *Found {
	for _, community := range s.Communities {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		params := &g.GoSNMP{
			Target:    host,
			Port:      s.Port,
			Community: community,
			Version:   g.Version2c,
			Timeout:   s.Timeout,
			Retries:   0,
		}

		if err := params.Connect(); err != nil {
			continue
		}
		defer params.Conn.Close()

		pkt, err := params.Get([]string{"1.3.6.1.2.1.1.1.0"}) // sysDescr.0
		if err != nil {
			continue
		}
		if len(pkt.Variables) == 0 {
			continue
		}
		v := pkt.Variables[0]
		if v.Type == g.NoSuchObject || v.Type == g.NoSuchInstance {
			continue
		}

		// Validate we got a real SNMP response (F-13).
		descr := fmt.Sprintf("%v", v.Value)
		return &Found{
			IP:        host,
			SysDescr:  descr,
			Community: community,
			Version:   "v2c",
		}
	}
	return nil
}

// hostsInCIDR enumerates all usable host IPs in a CIDR block.
func hostsInCIDR(cidr string) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Try treating it as a single host.
		if net.ParseIP(cidr) != nil {
			return []string{cidr}, nil
		}
		return nil, err
	}

	var hosts []string
	for ip = ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		hosts = append(hosts, ip.String())
	}

	// Remove network and broadcast addresses for IPv4.
	if strings.Contains(cidr, ".") && len(hosts) > 2 {
		hosts = hosts[1 : len(hosts)-1]
	}
	return hosts, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
