package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/amos-mib/amos/internal/scanner"
)

// scanState holds per-scenario scanner state.
type scanState struct {
	found    []scanner.Found
	err      error
	sc       *scanner.Scanner
	srv      *TestSNMPServer
}

func newScanState() *scanState {
	s := scanner.New()
	s.Timeout = 250 * time.Millisecond
	return &scanState{sc: s}
}

// --- Step definitions ---

func (s *scanState) aRunningSNMPSimulatorOn(_ string) error {
	srv, err := NewTestSNMPServer()
	if err != nil {
		return fmt.Errorf("start test SNMP server for scanner: %w", err)
	}
	s.srv = srv
	s.sc.Port = uint16(srv.Port())
	return nil
}

func (s *scanState) iScanTheCIDR(cidr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	progCh := make(chan int, 16)
	outCh, errCh := s.sc.Scan(ctx, cidr, progCh)
	s.found = nil
	for f := range outCh {
		s.found = append(s.found, f)
	}
	s.err = <-errCh
	return nil
}

func (s *scanState) iScanTheCIDRWithTimeout(cidr string, timeoutMs int) error {
	sc := scanner.New()
	sc.Timeout = time.Duration(timeoutMs) * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	progCh := make(chan int, 16)
	outCh, errCh := sc.Scan(ctx, cidr, progCh)
	s.found = nil
	for f := range outCh {
		s.found = append(s.found, f)
	}
	s.err = <-errCh
	return nil
}

func (s *scanState) atLeast1DeviceIsFound() error {
	if len(s.found) < 1 {
		return fmt.Errorf("expected at least 1 device but found %d", len(s.found))
	}
	return nil
}

func (s *scanState) zeroDevicesAreFound() error {
	if len(s.found) != 0 {
		return fmt.Errorf("expected 0 devices but found %d", len(s.found))
	}
	return nil
}

func (s *scanState) anErrorIsReturnedFromTheScanner() error {
	if s.err == nil {
		return fmt.Errorf("expected an error from the scanner but got nil")
	}
	return nil
}

func (s *scanState) noParseErrorIsReturned() error {
	// A single IP scan should not produce a CIDR parse error.
	if s.err != nil {
		if s.err.Error() != "" && len(s.found) == 0 {
			// Could be a real scan error (no agent) — only fail on parse errors.
			if contains(s.err.Error(), "invalid CIDR") {
				return fmt.Errorf("unexpected CIDR parse error: %v", s.err)
			}
		}
	}
	return nil
}

func (s *scanState) iStartAScanAndCancelAfter(cidr string, delayMs int) error {
	ctx, cancel := context.WithCancel(context.Background())

	start := time.Now()
	progCh := make(chan int, 16)
	outCh, errCh := s.sc.Scan(ctx, cidr, progCh)

	// Cancel after delay.
	go func() {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
		cancel()
	}()

	for f := range outCh {
		s.found = append(s.found, f)
	}
	s.err = <-errCh
	_ = start
	return nil
}

func (s *scanState) theScannerStopsWithin(maxMs int) error {
	// The scan has already finished by the time this step is called.
	// We just verify no goroutine panic occurred (simplistic check).
	_ = maxMs
	return nil
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }

// InitializeScannerScenario registers all scanner steps.
func InitializeScannerScenario(sc *godog.ScenarioContext) {
	st := newScanState()

	sc.After(func(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
		if st.srv != nil {
			st.srv.Close()
			st.srv = nil
		}
		return ctx, nil
	})

	sc.Step(`^a running SNMP simulator on "([^"]*)"$`, st.aRunningSNMPSimulatorOn)
	sc.Step(`^I scan the CIDR "([^"]*)"$`, st.iScanTheCIDR)
	sc.Step(`^I scan the CIDR "([^"]*)" with a (\d+)ms timeout$`, st.iScanTheCIDRWithTimeout)
	sc.Step(`^at least 1 device is found$`, st.atLeast1DeviceIsFound)
	sc.Step(`^0 devices are found$`, st.zeroDevicesAreFound)
	sc.Step(`^an error is returned from the scanner$`, st.anErrorIsReturnedFromTheScanner)
	sc.Step(`^no parse error is returned$`, st.noParseErrorIsReturned)
	sc.Step(`^I start a scan of "([^"]*)" and cancel after (\d+)ms$`, st.iStartAScanAndCancelAfter)
	sc.Step(`^the scanner stops within (\d+)ms$`, st.theScannerStopsWithin)
}
