package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/mib"
	"github.com/amos-mib/amos/internal/snmp"
)

// row holds the display values for one result entry.
type row struct {
	oid   string
	name  string
	typ   string
	value string
	errStr string
}

// ResultsTable displays SNMP operation results in a sortable table.
type ResultsTable struct {
	mu    sync.Mutex
	rows  []row
	table *widget.Table
	wrap  *container.Scroll
}

var tableHeaders = []string{"OID", "Name", "Type", "Value"}

// NewResultsTable creates an empty results table.
func NewResultsTable() *ResultsTable {
	rt := &ResultsTable{}
	rt.table = widget.NewTable(
		func() (int, int) {
			rt.mu.Lock()
			defer rt.mu.Unlock()
			return len(rt.rows) + 1, len(tableHeaders) // +1 for header row
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			rt.mu.Lock()
			defer rt.mu.Unlock()
			if id.Row == 0 {
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.SetText(tableHeaders[id.Col])
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			r := rt.rows[id.Row-1]
			switch id.Col {
			case 0:
				lbl.SetText(r.oid)
			case 1:
				if r.errStr != "" {
					lbl.SetText("⚠ " + r.errStr) // T-04
				} else {
					lbl.SetText(r.name)
				}
			case 2:
				lbl.SetText(r.typ)
			case 3:
				lbl.SetText(r.value)
			}
		},
	)
	rt.table.SetColumnWidth(0, 220)
	rt.table.SetColumnWidth(1, 220)
	rt.table.SetColumnWidth(2, 120)
	rt.table.SetColumnWidth(3, 300)

	rt.wrap = container.NewScroll(rt.table)
	return rt
}

// Clear removes all rows (F-15).
func (rt *ResultsTable) Clear() {
	rt.mu.Lock()
	rt.rows = nil
	rt.mu.Unlock()
	rt.table.Refresh()
}

// SetResults replaces all rows with the given results.
func (rt *ResultsTable) SetResults(results []snmp.Result, loader *mib.Loader) {
	rt.mu.Lock()
	rt.rows = make([]row, 0, len(results))
	for _, r := range results {
		rt.rows = append(rt.rows, toRow(r, loader))
	}
	rt.mu.Unlock()
	rt.table.Refresh()
}

// AppendResult adds a single result (used during streaming WALK).
func (rt *ResultsTable) AppendResult(r snmp.Result, loader *mib.Loader) {
	rt.mu.Lock()
	rt.rows = append(rt.rows, toRow(r, loader))
	rt.mu.Unlock()
	rt.table.Refresh()
}

func toRow(r snmp.Result, loader *mib.Loader) row {
	name := r.OID // numeric fallback (T-03)
	if loader != nil {
		if n := loader.ResolveOID(r.OID); n != nil && n.Name != n.NumericOID {
			name = n.Name
		}
	}
	return row{
		oid:    r.OID,
		name:   name,
		typ:    r.Type,
		value:  fmt.Sprintf("%v", r.Value),
		errStr: r.Error,
	}
}

// Container returns the Fyne container for embedding.
func (rt *ResultsTable) Container() fyne.CanvasObject {
	return rt.wrap
}
