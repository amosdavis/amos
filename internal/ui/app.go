// Package ui contains all Fyne UI components for AMOS.
package ui

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/mib"
	"github.com/amos-mib/amos/internal/scanner"
	"github.com/amos-mib/amos/internal/snmp"
)

// connState represents the SNMP connection indicator state.
type connState int

const (
	connIdle connState = iota
	connOK
	connFail
	connBusy
)

// App is the root application controller.
type App struct {
	fyneApp   fyne.App
	win       fyne.Window
	loader    *mib.Loader
	mibTree   *MIBTree
	results   *ResultsTable
	detail    *DetailPanel
	toolbar   *Toolbar
	statusBar *widget.Label
	activity  *widget.Activity
	cancelOp  context.CancelFunc
}

// NewApp creates and wires all UI components.
func NewApp(bundleMIBDir string) *App {
	a := &App{
		fyneApp: app.NewWithID("com.amos-mib.amos"),
		loader:  mib.NewLoader(bundleMIBDir),
	}
	a.win = a.fyneApp.NewWindow("AMOS — Amos MIB Operating System")
	a.win.Resize(fyne.NewSize(1200, 700))
	a.win.SetMaster()

	a.results = NewResultsTable()
	a.detail = NewDetailPanel()
	a.statusBar = widget.NewLabel("Ready.")
	a.statusBar.Wrapping = fyne.TextTruncate
	a.activity = widget.NewActivity()

	a.mibTree = NewMIBTree(func(n *mib.Node) {
		a.detail.SetNode(n)
		if a.toolbar != nil {
			a.toolbar.SetOID(n.NumericOID)
		}
	})

	a.toolbar = NewToolbar(a)
	a.buildLayout()

	// Load MIBs in background after window is visible.
	go a.loadMIBs()

	return a
}

func (a *App) buildLayout() {
	// Three-panel split: MIB tree | results | detail.
	rightSplit := container.NewHSplit(a.results.Container(), a.detail.Container())
	rightSplit.SetOffset(0.65)

	mainSplit := container.NewHSplit(a.mibTree.Container(), rightSplit)
	mainSplit.SetOffset(0.25)

	statusRow := container.NewBorder(nil, nil, a.activity, nil, a.statusBar)
	content := container.NewBorder(
		a.toolbar.Container(),
		statusRow,
		nil, nil,
		mainSplit,
	)
	a.win.SetContent(content)
}

func (a *App) loadMIBs() {
	a.setStatus("Loading MIBs…")
	errs := a.loader.Load()
	tree := a.loader.BuildTree()
	a.mibTree.SetTree(tree)
	if len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, fmt.Sprintf("%s: %v", e.File, e.Err))
		}
		a.setStatus(fmt.Sprintf("MIBs loaded with %d error(s). See Tools > MIB Errors.", len(errs)))
		_ = msgs // available for a future dialog
	} else {
		a.setStatus("MIBs loaded.")
	}
}

// Run shows the window and blocks until the application exits.
func (a *App) Run() {
	a.win.ShowAndRun()
}

func (a *App) setStatus(msg string) {
	fyne.Do(func() { a.statusBar.SetText(msg) })
}

func (a *App) setActive(active bool) {
	fyne.Do(func() {
		if active {
			a.activity.Start()
		} else {
			a.activity.Stop()
		}
	})
}

// Connect tests connectivity to the configured host by fetching sysDescr.0.
func (a *App) Connect() {
	target := a.toolbar.target()
	if strings.TrimSpace(target.Host) == "" {
		a.setStatus("Enter a host before connecting.")
		return
	}
	a.toolbar.SetConnectionStatus(connBusy, "Connecting…")
	a.setStatus("Connecting to " + target.Host + "…")
	a.setActive(true)

	go func() {
		defer a.setActive(false)
		ctx, cancel := context.WithTimeout(context.Background(), 10*snmp.DefaultTimeout)
		defer cancel()
		client := snmp.GetClient(target)
		results, err := client.Get(ctx, []string{"1.3.6.1.2.1.1.1.0"})
		if err != nil {
			a.toolbar.SetConnectionStatus(connFail, "Failed")
			a.setStatus("Connection failed: " + err.Error())
			return
		}
		sysDescr := ""
		if len(results) > 0 {
			sysDescr = fmt.Sprintf("%v", results[0].Value)
			if len(sysDescr) > 60 {
				sysDescr = sysDescr[:60] + "…"
			}
		}
		a.toolbar.SetConnectionStatus(connOK, target.Host)
		a.setStatus(fmt.Sprintf("Connected — %s", sysDescr))
	}()
}

// LoadMIBFile opens a file dialog and adds the chosen MIB file's directory.
func (a *App) LoadMIBFile() {
	dialog.ShowFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		dir := filepath.Dir(r.URI().Path())
		a.loader.AddPath(dir)
		go a.loadMIBs()
	}, a.win)
}

// ExecGet runs a GET for the OID shown in the toolbar.
func (a *App) ExecGet(target snmp.Target, oid string) {
	if err := validateInputs(target, oid); err != nil {
		a.setStatus("Input error: " + err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*snmp.DefaultTimeout)
	a.cancelOp = cancel
	a.results.Clear()
	a.setStatus("GET " + oid + " …")
	a.setActive(true)

	go func() {
		defer a.setActive(false)
		defer cancel()
		client := snmp.GetClient(target)
		results, err := client.Get(ctx, []string{oid})
		if err != nil {
			a.setStatus("GET error: " + err.Error())
			return
		}
		a.results.SetResults(results, a.loader)
		a.setStatus(fmt.Sprintf("GET returned %d binding(s).", len(results)))
	}()
}

// ExecGetNext runs a GETNEXT for the OID shown in the toolbar.
func (a *App) ExecGetNext(target snmp.Target, oid string) {
	if err := validateInputs(target, oid); err != nil {
		a.setStatus("Input error: " + err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*snmp.DefaultTimeout)
	a.cancelOp = cancel
	a.results.Clear()
	a.setStatus("GETNEXT " + oid + " …")
	a.setActive(true)

	go func() {
		defer a.setActive(false)
		defer cancel()
		client := snmp.GetClient(target)
		results, err := client.GetNext(ctx, []string{oid})
		if err != nil {
			a.setStatus("GETNEXT error: " + err.Error())
			return
		}
		a.results.SetResults(results, a.loader)
		a.setStatus(fmt.Sprintf("GETNEXT returned %d binding(s).", len(results)))
	}()
}

// ExecGetBulk runs a GETBULK with maxReps=10 (T-06).
func (a *App) ExecGetBulk(target snmp.Target, oid string) {
	if err := validateInputs(target, oid); err != nil {
		a.setStatus("Input error: " + err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*snmp.DefaultTimeout)
	a.cancelOp = cancel
	a.results.Clear()
	a.setStatus("GETBULK " + oid + " …")
	a.setActive(true)

	go func() {
		defer a.setActive(false)
		defer cancel()
		client := snmp.GetClient(target)
		results, err := client.GetBulk(ctx, []string{oid}, 0, snmp.DefaultMaxReps)
		if err != nil {
			a.setStatus("GETBULK error: " + err.Error())
			return
		}
		a.results.SetResults(results, a.loader)
		a.setStatus(fmt.Sprintf("GETBULK returned %d binding(s).", len(results)))
	}()
}

// ExecWalk walks the subtree rooted at oid.
func (a *App) ExecWalk(target snmp.Target, oid string) {
	if err := validateInputs(target, oid); err != nil {
		a.setStatus("Input error: " + err.Error())
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelOp = cancel
	a.results.Clear()
	a.setStatus("WALK " + oid + " …")
	a.setActive(true)

	go func() {
		defer a.setActive(false)
		defer cancel()
		client := snmp.GetClient(target)
		ch, errCh := client.Walk(ctx, oid)
		count := 0
		for r := range ch {
			a.results.AppendResult(r, a.loader)
			count++
		}
		if err := <-errCh; err != nil && err != context.Canceled {
			a.setStatus(fmt.Sprintf("WALK error: %v", err))
			return
		}
		a.setStatus(fmt.Sprintf("WALK complete — %d binding(s).", count))
	}()
}

// ExecSet opens a dialog asking for value + type, then sends a SET.
func (a *App) ExecSet(target snmp.Target, oid string) {
	if err := validateInputs(target, oid); err != nil {
		a.setStatus("Input error: " + err.Error())
		return
	}
	ShowSetDialog(a.win, oid, func(pdu []interface{}) {
		// pdu carries value as string; convert in toolbar handler.
		_ = pdu
	}, func(pduList []snmpSetPDU) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*snmp.DefaultTimeout)
		a.cancelOp = cancel
		a.results.Clear()
		a.setStatus("SET " + oid + " …")
		a.setActive(true)

		go func() {
			defer a.setActive(false)
			defer cancel()
			client := snmp.GetClient(target)
			goPDUs := toGoPDUs(pduList)
			results, err := client.Set(ctx, goPDUs)
			if err != nil {
				a.setStatus("SET error: " + err.Error())
				return
			}
			a.results.SetResults(results, a.loader)
			a.setStatus(fmt.Sprintf("SET returned %d binding(s).", len(results)))
		}()
	})
}

// CancelOp cancels the currently running SNMP operation.
func (a *App) CancelOp() {
	if a.cancelOp != nil {
		a.cancelOp()
		a.setStatus("Operation cancelled.")
	}
}

// ShowNetworkScanner opens the subnet scanner dialog.
func (a *App) ShowNetworkScanner(defaultCIDR string) {
	ShowScannerDialog(a.win, defaultCIDR, scanner.New(), func(f scanner.Found) {
		a.toolbar.SetHost(f.IP)
		a.toolbar.SetCommunity(f.Community)
		a.setStatus(fmt.Sprintf("Target set to %s (%s)", f.IP, f.SysDescr))
	})
}

// DetectLocalSubnet returns the first non-loopback IPv4 CIDR detected.
func DetectLocalSubnet() string {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return "192.168.1.0/24"
	}
	for _, addr := range ifaces {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
				continue
			}
			return ipNet.String()
		}
	}
	return "192.168.1.0/24"
}

// DefaultMIBDir returns the bundled MIBs directory relative to the binary.
func DefaultMIBDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "mibs"
	}
	dir := filepath.Join(filepath.Dir(exe), "mibs")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	// Development fallback: look for mibs/ relative to the module root.
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "mibs")
	if _, err := os.Stat(root); err == nil {
		return root
	}
	return "mibs"
}

func validateInputs(target snmp.Target, oid string) error {
	if strings.TrimSpace(target.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if strings.TrimSpace(oid) == "" {
		return fmt.Errorf("OID is required")
	}
	return nil
}
