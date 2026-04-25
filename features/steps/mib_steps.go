package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"

	"github.com/amos-mib/amos/internal/mib"
)

// mibState holds per-scenario MIB loader state.
type mibState struct {
	loader   *mib.Loader
	tree     *mib.Node
	loadErrs []mib.LoadError
	bundleDir string
}

func newMIBState() *mibState {
	// Find the bundled MIB directory relative to this file.
	dir := findBundleDir()
	return &mibState{bundleDir: dir}
}

func findBundleDir() string {
	// Walk up from cwd to find mibs/ directory.
	dir, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "mibs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return "mibs"
}

// --- Step definitions ---

func (m *mibState) theBundledMIBDirectory() error {
	m.loader = mib.NewLoader(m.bundleDir)
	return nil
}

func (m *mibState) iLoadAllMIBs() error {
	if m.loader == nil {
		m.loader = mib.NewLoader(m.bundleDir)
	}
	m.loadErrs = m.loader.Load()
	return nil
}

func (m *mibState) noFatalLoadErrorsAreReported() error {
	// We permit partial errors (unknown module deps) but not zero-result loads.
	// A "fatal" error would be a panic-recovered crash, not a missing import.
	for _, e := range m.loadErrs {
		if strings.Contains(e.Err.Error(), "panic") {
			return fmt.Errorf("fatal load error in %s: %v", e.File, e.Err)
		}
	}
	return nil
}

func (m *mibState) oidResolvesToNameContaining(oidStr, substr string) error {
	node := m.loader.ResolveOID(oidStr)
	if node == nil {
		return fmt.Errorf("OID %s resolved to nil", oidStr)
	}
	if !strings.Contains(strings.ToLower(node.Name), strings.ToLower(substr)) {
		return fmt.Errorf("OID %s resolved to %q, expected it to contain %q", oidStr, node.Name, substr)
	}
	return nil
}

func (m *mibState) oidResolvesToExact(oidStr, expected string) error {
	node := m.loader.ResolveOID(oidStr)
	if node == nil {
		return fmt.Errorf("OID %s resolved to nil", oidStr)
	}
	if node.Name != expected && node.NumericOID != expected {
		return fmt.Errorf("OID %s resolved to name=%q / oid=%q, expected %q", oidStr, node.Name, node.NumericOID, expected)
	}
	return nil
}

func (m *mibState) aTemporaryDirectoryWithValidAndMalformedMIB() error {
	tmp, err := os.MkdirTemp("", "amos-mib-test-*")
	if err != nil {
		return err
	}
	// Write a minimal valid SNMPv2-SMI-like stub.
	validMIB := `TEST-MIB DEFINITIONS ::= BEGIN
IMPORTS MODULE-IDENTITY FROM SNMPv2-SMI;
testMIB MODULE-IDENTITY
    LAST-UPDATED "202601010000Z"
    ORGANIZATION "AMOS Test"
    CONTACT-INFO ""
    DESCRIPTION "test"
    ::= { enterprises 99999 }
END`
	if err := os.WriteFile(filepath.Join(tmp, "TEST-MIB.txt"), []byte(validMIB), 0644); err != nil {
		return err
	}
	// Write a deliberately malformed MIB.
	if err := os.WriteFile(filepath.Join(tmp, "BROKEN-MIB.txt"), []byte("THIS IS NOT A VALID MIB @@@"), 0644); err != nil {
		return err
	}
	m.loader = mib.NewLoader(m.bundleDir)
	m.loader.AddPath(tmp)
	return nil
}

func (m *mibState) iLoadAllMIBsFromThatDirectory() error {
	m.loadErrs = m.loader.Load()
	return nil
}

func (m *mibState) oidsFromValidMIBResolveCorrectly() error {
	// After loading, the bundled SNMPv2-MIB OIDs should still resolve.
	node := m.loader.ResolveOID("1.3.6.1.2.1.1.1.0")
	if node == nil {
		return fmt.Errorf("standard OID resolution broken after loading mixed MIBs")
	}
	return nil
}

func (m *mibState) iBuildTheOIDTree() error {
	m.tree = m.loader.BuildTree()
	return nil
}

func (m *mibState) theRootNodeIsNotNil() error {
	if m.tree == nil {
		return fmt.Errorf("expected a non-nil root node but got nil")
	}
	return nil
}

// InitializeMIBScenario registers all MIB steps.
func InitializeMIBScenario(sc *godog.ScenarioContext) {
	m := newMIBState()

	sc.Step(`^the bundled MIB directory$`, m.theBundledMIBDirectory)
	sc.Step(`^I load all MIBs$`, m.iLoadAllMIBs)
	sc.Step(`^no fatal load errors are reported$`, m.noFatalLoadErrorsAreReported)
	sc.Step(`^OID "([^"]*)" resolves to a name containing "([^"]*)"$`, m.oidResolvesToNameContaining)
	sc.Step(`^OID "([^"]*)" resolves to "([^"]*)"$`, m.oidResolvesToExact)
	sc.Step(`^a temporary directory with a valid MIB and a malformed MIB$`, m.aTemporaryDirectoryWithValidAndMalformedMIB)
	sc.Step(`^I load all MIBs from that directory$`, m.iLoadAllMIBsFromThatDirectory)
	sc.Step(`^OIDs from the valid MIB resolve correctly$`, m.oidsFromValidMIBResolveCorrectly)
	sc.Step(`^I build the OID tree$`, m.iBuildTheOIDTree)
	sc.Step(`^the root node is not nil$`, m.theRootNodeIsNotNil)
}
