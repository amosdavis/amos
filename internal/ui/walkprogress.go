package ui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// WalkProgress shows a live progress bar and binding counter during WALK operations.
// The container is empty (zero height) when idle and populates during a walk.
type WalkProgress struct {
	mu         sync.Mutex
	bar        *widget.ProgressBarInfinite
	label      *widget.Label
	wrap       *fyne.Container
	lastUpdate time.Time
	active     bool
}

// NewWalkProgress creates a WalkProgress (initially empty/hidden).
func NewWalkProgress() *WalkProgress {
	wp := &WalkProgress{
		bar:   widget.NewProgressBarInfinite(),
		label: widget.NewLabel(""),
	}
	wp.wrap = container.NewVBox() // empty → zero height when idle
	return wp
}

// Start shows the progress bar for a walk of the given root OID.
func (wp *WalkProgress) Start(rootOID string) {
	wp.mu.Lock()
	wp.active = true
	wp.lastUpdate = time.Time{} // force first update through
	wp.mu.Unlock()

	fyne.Do(func() {
		wp.label.SetText(fmt.Sprintf("Walking %s — 0 bindings received…", rootOID))
		wp.wrap.Objects = []fyne.CanvasObject{wp.bar, wp.label}
		wp.bar.Start()
		wp.wrap.Refresh()
	})
}

// Update refreshes the binding count and last OID. Throttled to ~10 redraws/sec
// to avoid flooding the UI thread during large walks.
func (wp *WalkProgress) Update(count int, currentOID string) {
	wp.mu.Lock()
	if !wp.active {
		wp.mu.Unlock()
		return
	}
	now := time.Now()
	if count > 5 && now.Sub(wp.lastUpdate) < 100*time.Millisecond {
		wp.mu.Unlock()
		return // throttle
	}
	wp.lastUpdate = now
	wp.mu.Unlock()

	fyne.Do(func() {
		wp.label.SetText(fmt.Sprintf(
			"Walking — %d bindings received  |  Last: %s", count, currentOID,
		))
	})
}

// Stop hides the progress bar after the walk completes or is cancelled.
func (wp *WalkProgress) Stop() {
	wp.mu.Lock()
	wp.active = false
	wp.mu.Unlock()

	fyne.Do(func() {
		wp.bar.Stop()
		wp.wrap.Objects = nil
		wp.wrap.Refresh()
	})
}

// Container returns the Fyne object for layout embedding.
func (wp *WalkProgress) Container() fyne.CanvasObject {
	return wp.wrap
}
