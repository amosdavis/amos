package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/mib"
)

// DetailPanel shows metadata for the currently selected MIB node.
type DetailPanel struct {
	nameLabel   *widget.Label
	oidLabel    *widget.Label
	syntaxLabel *widget.Label
	accessLabel *widget.Label
	descLabel   *widget.Label
	wrap        *container.Scroll
}

// NewDetailPanel creates an empty detail panel.
func NewDetailPanel() *DetailPanel {
	d := &DetailPanel{
		nameLabel:   widget.NewLabel(""),
		oidLabel:    widget.NewLabel(""),
		syntaxLabel: widget.NewLabel(""),
		accessLabel: widget.NewLabel(""),
		descLabel:   widget.NewLabel(""),
	}
	d.descLabel.Wrapping = fyne.TextWrapWord

	form := container.NewVBox(
		sectionHeader("OID Detail"),
		labeledRow("Name:", d.nameLabel),
		labeledRow("OID:", d.oidLabel),
		labeledRow("Syntax:", d.syntaxLabel),
		labeledRow("Access:", d.accessLabel),
		widget.NewSeparator(),
		widget.NewLabel("Description:"),
		d.descLabel,
	)

	d.wrap = container.NewScroll(form)
	return d
}

// SetNode updates the panel to display the given MIB node.
func (d *DetailPanel) SetNode(n *mib.Node) {
	if n == nil {
		d.nameLabel.SetText("")
		d.oidLabel.SetText("")
		d.syntaxLabel.SetText("")
		d.accessLabel.SetText("")
		d.descLabel.SetText("")
		return
	}
	d.nameLabel.SetText(n.Name)
	d.oidLabel.SetText(n.NumericOID)
	d.syntaxLabel.SetText(n.Syntax)
	d.accessLabel.SetText(n.Access)
	d.descLabel.SetText(n.Description)
}

// Container returns the Fyne container for embedding.
func (d *DetailPanel) Container() fyne.CanvasObject {
	return d.wrap
}

func sectionHeader(text string) *widget.Label {
	lbl := widget.NewLabel(text)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return lbl
}

func labeledRow(label string, value *widget.Label) *fyne.Container {
	lbl := widget.NewLabel(label)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	return container.NewHBox(lbl, value)
}
