package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/scanner"
)

// ShowScannerDialog opens a modal network scanner dialog.
// onSelect is called when the user double-clicks a discovered host.
func ShowScannerDialog(win fyne.Window, defaultCIDR string, sc *scanner.Scanner, onSelect func(scanner.Found)) {
	cidrEntry := widget.NewEntry()
	cidrEntry.SetText(defaultCIDR)

	progress := widget.NewProgressBar()
	progress.Hide()

	statusLbl := widget.NewLabel("Enter a CIDR range and press Scan.")

	var found []scanner.Found
	list := widget.NewList(
		func() int { return len(found) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			f := found[id]
			obj.(*widget.Label).SetText(fmt.Sprintf("%s  [%s]  %s", f.IP, f.Community, truncate(f.SysDescr, 60)))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if onSelect != nil {
			onSelect(found[id])
		}
	}

	var cancelScan context.CancelFunc

	scanBtn := widget.NewButton("Scan", nil)
	cancelBtn := widget.NewButton("Cancel Scan", func() {
		if cancelScan != nil {
			cancelScan()
		}
	})
	cancelBtn.Disable()

	scanBtn.OnTapped = func() {
		cidr := cidrEntry.Text
		if err := validateCIDR(cidr); err != nil {
			dialog.ShowError(fmt.Errorf("invalid CIDR: %w", err), win)
			return
		}

		found = nil
		list.Refresh()
		progress.SetValue(0)
		progress.Show()
		scanBtn.Disable()
		cancelBtn.Enable()
		statusLbl.SetText("Scanning…")

		ctx, cancel := context.WithCancel(context.Background())
		cancelScan = cancel

		progCh := make(chan int, 8)
		outCh, errCh := sc.Scan(ctx, cidr, progCh)

		go func() {
			for f := range outCh {
				found = append(found, f)
				list.Refresh()
				statusLbl.SetText(fmt.Sprintf("Found %d device(s)…", len(found)))
			}
			if err := <-errCh; err != nil && err != context.Canceled {
				statusLbl.SetText("Scan error: " + err.Error())
			} else if err == context.Canceled {
				statusLbl.SetText(fmt.Sprintf("Scan cancelled. Found %d device(s).", len(found)))
			} else {
				statusLbl.SetText(fmt.Sprintf("Scan complete. Found %d device(s).", len(found)))
			}
			progress.Hide()
			scanBtn.Enable()
			cancelBtn.Disable()
		}()

		go func() {
			for pct := range progCh {
				progress.SetValue(float64(pct) / 100.0)
			}
		}()
	}

	content := container.NewBorder(
		container.NewVBox(
			container.NewHBox(widget.NewLabel("CIDR:"), cidrEntry, scanBtn, cancelBtn),
			progress,
			statusLbl,
		),
		widget.NewLabel("Double-click a host to use it as the AMOS target."),
		nil, nil,
		list,
	)

	d := dialog.NewCustom("Network Scanner", "Close", content, win)
	d.Resize(fyne.NewSize(700, 420))
	d.Show()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func validateCIDR(cidr string) error {
	_, _, err := parseCIDROrIP(cidr)
	return err
}
