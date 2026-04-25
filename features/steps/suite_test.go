package steps_test

import (
	"os"
	"testing"

	"github.com/cucumber/godog"

	"github.com/amos-mib/amos/features/steps"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			steps.InitializeSNMPScenario(sc)
			steps.InitializeMIBScenario(sc)
			steps.InitializeScannerScenario(sc)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

// TestMain sets the working directory to the project root so relative paths
// (mibs/, features/) resolve correctly.
func TestMain(m *testing.M) {
	// Walk up from the test binary location to find go.mod.
	_ = os.Chdir("../..")
	os.Exit(m.Run())
}
