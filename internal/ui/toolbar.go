package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/snmp"
)

// Toolbar is the top connection bar + operation buttons.
type Toolbar struct {
	app       *App
	hostEntry *widget.Entry
	oidEntry  *widget.Entry
	commEntry *widget.Entry
	verSelect *widget.Select
	wrap      *fyne.Container
}

// NewToolbar creates the toolbar and wires button callbacks.
func NewToolbar(a *App) *Toolbar {
	t := &Toolbar{app: a}

	t.hostEntry = widget.NewEntry()
	t.hostEntry.SetPlaceHolder("Host / IP")
	t.hostEntry.SetText("demo.snmplabs.com")

	t.commEntry = widget.NewEntry()
	t.commEntry.SetPlaceHolder("Community")
	t.commEntry.SetText("public")

	t.oidEntry = widget.NewEntry()
	t.oidEntry.SetPlaceHolder("OID (e.g. 1.3.6.1.2.1.1 or sysDescr)")
	t.oidEntry.SetText("1.3.6.1.2.1.1")

	t.verSelect = widget.NewSelect([]string{"v1", "v2c", "v3"}, nil)
	t.verSelect.SetSelected("v2c")

	connectBtn := widget.NewButtonWithIcon("Connect", theme.LoginIcon(), func() {
		a.setStatus(fmt.Sprintf("Target: %s  community: %s  version: %s",
			t.hostEntry.Text, t.commEntry.Text, t.verSelect.Selected))
	})

	getBtn := widget.NewButtonWithIcon("GET", theme.SearchIcon(), func() {
		a.ExecGet(t.target(), t.oidEntry.Text)
	})
	getNextBtn := widget.NewButton("GETNEXT", func() {
		a.ExecGetNext(t.target(), t.oidEntry.Text)
	})
	getBulkBtn := widget.NewButton("GETBULK", func() {
		a.ExecGetBulk(t.target(), t.oidEntry.Text)
	})
	walkBtn := widget.NewButtonWithIcon("WALK", theme.ViewRefreshIcon(), func() {
		a.ExecWalk(t.target(), t.oidEntry.Text)
	})
	setBtn := widget.NewButtonWithIcon("SET", theme.DocumentSaveIcon(), func() {
		a.ExecSet(t.target(), t.oidEntry.Text)
	})
	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		a.CancelOp()
	})
	scanBtn := widget.NewButtonWithIcon("Scan Network", theme.ComputerIcon(), func() {
		a.ShowNetworkScanner(DetectLocalSubnet())
	})
	loadMIBBtn := widget.NewButtonWithIcon("Load MIB…", theme.FolderOpenIcon(), func() {
		a.LoadMIBFile()
	})

	connectionRow := container.NewBorder(
		nil, nil,
		container.NewHBox(widget.NewLabel("Host:")),
		container.NewHBox(widget.NewLabel("Ver:"), t.verSelect, widget.NewLabel("Community:"), t.commEntry, connectBtn),
		t.hostEntry,
	)
	oidRow := container.NewBorder(
		nil, nil,
		widget.NewLabel("OID:"),
		nil,
		t.oidEntry,
	)
	opRow := container.NewHBox(
		getBtn, getNextBtn, getBulkBtn, walkBtn, setBtn, cancelBtn,
		widget.NewSeparator(),
		scanBtn, loadMIBBtn,
	)

	t.wrap = container.NewVBox(
		connectionRow,
		oidRow,
		opRow,
		widget.NewSeparator(),
	)
	return t
}

// SetHost updates the host field (called from scanner dialog).
func (t *Toolbar) SetHost(host string) {
	t.hostEntry.SetText(host)
}

// SetCommunity updates the community field.
func (t *Toolbar) SetCommunity(community string) {
	t.commEntry.SetText(community)
}

// target builds a snmp.Target from the current toolbar values.
func (t *Toolbar) target() snmp.Target {
	ver := snmp.Version2c
	switch t.verSelect.Selected {
	case "v1":
		ver = snmp.Version1
	case "v3":
		ver = snmp.Version3
	}
	return snmp.Target{
		Host:      strings.TrimSpace(t.hostEntry.Text),
		Port:      161,
		Version:   ver,
		Community: strings.TrimSpace(t.commEntry.Text),
		Retries:   1,
	}
}

// Container returns the Fyne container for embedding.
func (t *Toolbar) Container() fyne.CanvasObject {
	return t.wrap
}

// snmpSetPDU carries a single SET request item before conversion to gosnmp types.
type snmpSetPDU struct {
	OID   string
	Type  string
	Value string
}

// ShowSetDialog opens a dialog for entering SET value + type.
func ShowSetDialog(win fyne.Window, oid string, _ func([]interface{}), onOK func([]snmpSetPDU)) {
	valueEntry := widget.NewEntry()
	valueEntry.SetPlaceHolder("Value")
	typeSelect := widget.NewSelect([]string{"OctetString", "Integer", "OID", "IPAddress", "Counter32", "Gauge32", "TimeTicks"}, nil)
	typeSelect.SetSelected("OctetString")

	form := widget.NewForm(
		widget.NewFormItem("OID", widget.NewLabel(oid)),
		widget.NewFormItem("Type", typeSelect),
		widget.NewFormItem("Value", valueEntry),
	)

	d := &widget.PopUp{}
	okBtn := widget.NewButton("SET", func() {
		d.Hide()
		onOK([]snmpSetPDU{{OID: oid, Type: typeSelect.Selected, Value: valueEntry.Text}})
	})
	cancelBtn := widget.NewButton("Cancel", func() { d.Hide() })

	content := container.NewVBox(
		widget.NewLabel("SET Operation"),
		form,
		container.NewHBox(okBtn, cancelBtn),
	)
	d = widget.NewModalPopUp(content, win.Canvas())
	d.Show()
}
