package steps

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"

	"github.com/amos-mib/amos/internal/snmp"
)

// snmpState holds per-scenario SNMP state.
type snmpState struct {
	target  snmp.Target
	results []snmp.Result
	err     error
	walkOut []snmp.Result
	server  *TestSNMPServer
}

func newSNMPState() *snmpState { return &snmpState{} }

// --- Step definitions ---

func (s *snmpState) aLocalTestSNMPServer() error {
	srv, err := NewTestSNMPServer()
	if err != nil {
		return fmt.Errorf("start test SNMP server: %w", err)
	}
	s.server = srv
	s.target = snmp.Target{
		Host:      "127.0.0.1",
		Port:      uint16(srv.Port()),
		Version:   snmp.Version2c,
		Community: "public",
		Retries:   0,
	}
	return nil
}

func (s *snmpState) anSNMPV2cTargetAtWithCommunity(host, community string) error {
	s.target = snmp.Target{
		Host:      host,
		Port:      161,
		Version:   snmp.Version2c,
		Community: community,
		Retries:   0,
	}
	return nil
}

func (s *snmpState) anSNMPV1TargetAtWithCommunity(host, community string) error {
	s.target = snmp.Target{
		Host:      host,
		Port:      161,
		Version:   snmp.Version1,
		Community: community,
		Retries:   0,
	}
	return nil
}

func (s *snmpState) iPerformGETOnOID(oid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := snmp.GetClient(s.target)
	s.results, s.err = client.Get(ctx, []string{oid})
	return nil
}

func (s *snmpState) iPerformGETOnOIDWithTimeout(oid string, timeoutMs int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	client := snmp.GetClient(s.target)
	s.results, s.err = client.Get(ctx, []string{oid})
	return nil
}

func (s *snmpState) iPerformGETNEXTOnOID(oid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := snmp.GetClient(s.target)
	s.results, s.err = client.GetNext(ctx, []string{oid})
	return nil
}

func (s *snmpState) iPerformGETBULKOnOID(oid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := snmp.GetClient(s.target)
	s.results, s.err = client.GetBulk(ctx, []string{oid}, 0, snmp.DefaultMaxReps)
	return nil
}

func (s *snmpState) iPerformWALKOnOID(oid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := snmp.GetClient(s.target)
	ch, errCh := client.Walk(ctx, oid)
	s.walkOut = nil
	for r := range ch {
		s.walkOut = append(s.walkOut, r)
	}
	s.err = <-errCh
	return nil
}

func (s *snmpState) iValidateTargetAndOID(oid string) error {
	if strings.TrimSpace(s.target.Host) == "" {
		s.err = fmt.Errorf("host is required")
		return nil
	}
	if strings.TrimSpace(oid) == "" {
		s.err = fmt.Errorf("OID is required")
		return nil
	}
	s.err = nil
	return nil
}

func (s *snmpState) theResultListIsNotEmpty() error {
	if s.err != nil {
		return fmt.Errorf("unexpected error: %v", s.err)
	}
	if len(s.results) == 0 {
		return fmt.Errorf("expected non-empty results but got none")
	}
	return nil
}

func (s *snmpState) noResultHasAnError() error {
	for _, r := range s.results {
		if r.Error != "" {
			return fmt.Errorf("result %s has error: %s", r.OID, r.Error)
		}
	}
	return nil
}

func (s *snmpState) anErrorIsReturnedContaining(substr string) error {
	if s.err == nil {
		return fmt.Errorf("expected an error containing %q but got nil", substr)
	}
	if !strings.Contains(s.err.Error(), substr) {
		return fmt.Errorf("expected error containing %q but got %q", substr, s.err.Error())
	}
	return nil
}

func (s *snmpState) anErrorIsReturned() error {
	if s.err == nil {
		return fmt.Errorf("expected an error but got nil (results: %v)", s.results)
	}
	return nil
}

func (s *snmpState) moreThanZeroBindingsCollected() error {
	if s.err != nil && s.err != context.DeadlineExceeded && s.err != context.Canceled {
		return fmt.Errorf("walk returned error: %v", s.err)
	}
	if len(s.walkOut) == 0 {
		return fmt.Errorf("expected more than 0 walk bindings")
	}
	return nil
}

func (s *snmpState) aValidationErrorIsReturned() error {
	if s.err == nil {
		return fmt.Errorf("expected a validation error but got nil")
	}
	return nil
}

// InitializeSNMPScenario registers all SNMP steps.
func InitializeSNMPScenario(sc *godog.ScenarioContext) {
	s := newSNMPState()

	sc.After(func(ctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if s.server != nil {
			s.server.Close()
			s.server = nil
		}
		return ctx, nil
	})

	sc.Step(`^a local test SNMP server$`, s.aLocalTestSNMPServer)
	sc.Step(`^an SNMP v2c target at "([^"]*)" with community "([^"]*)"$`, s.anSNMPV2cTargetAtWithCommunity)
	sc.Step(`^an SNMP v1 target at "([^"]*)" with community "([^"]*)"$`, s.anSNMPV1TargetAtWithCommunity)
	sc.Step(`^I perform GET on OID "([^"]*)"$`, s.iPerformGETOnOID)
	sc.Step(`^I perform GET on OID "([^"]*)" with a (\d+)ms timeout$`, s.iPerformGETOnOIDWithTimeout)
	sc.Step(`^I perform GETNEXT on OID "([^"]*)"$`, s.iPerformGETNEXTOnOID)
	sc.Step(`^I perform GETBULK on OID "([^"]*)"$`, s.iPerformGETBULKOnOID)
	sc.Step(`^I perform WALK on OID "([^"]*)"$`, s.iPerformWALKOnOID)
	sc.Step(`^I validate the target and OID "([^"]*)"$`, s.iValidateTargetAndOID)
	sc.Step(`^the result list is not empty$`, s.theResultListIsNotEmpty)
	sc.Step(`^no result has an error$`, s.noResultHasAnError)
	sc.Step(`^an error is returned containing "([^"]*)"$`, s.anErrorIsReturnedContaining)
	sc.Step(`^an error is returned$`, s.anErrorIsReturned)
	sc.Step(`^more than 0 bindings are collected$`, s.moreThanZeroBindingsCollected)
	sc.Step(`^a validation error is returned$`, s.aValidationErrorIsReturned)
}

// Ensure this compiles with testing.
var _ *testing.T

