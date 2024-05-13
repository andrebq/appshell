package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type (
	win struct {
		widget  fyne.Window
		output  binding.String
		nextCmd binding.String

		history history

		ctx context.Context
		sh  Shell
	}

	Shell interface {
		Snapshot(ctx context.Context, out io.Writer) error
		RestoreSnapshot(ctx context.Context, in io.Reader) error
		Parse(ctx context.Context, code string) (string, error)
		Eval(ctx context.Context, stdout, stderr io.Writer, code string, in io.Reader) error
	}

	emptyBuffer struct{}
)

func (emptyBuffer) Read(out []byte) (int, error) {
	return 0, io.EOF
}

func (w *win) evalCmd(updateHistory bool) {
	combined := strings.Builder{}

	cmd, _ := w.nextCmd.Get()

	cmd, err := w.sh.Parse(w.ctx, cmd)
	if err != nil {
		w.showError(err)
		return
	}
	if len(cmd) == 0 {
		return
	}
	combined.WriteString(cmd)
	combined.WriteString("\n---\n")

	err = w.sh.Eval(w.ctx, &combined, &combined, cmd, emptyBuffer{})
	if err != nil {
		fmt.Fprintf(&combined, "%v", err)
	} else {
		if updateHistory {
			w.history.add(cmd)
		}
		w.nextCmd.Set("")
	}

	previous, _ := w.output.Get()
	if previous == "" {
		w.output.Set(combined.String())
	} else {
		w.output.Set(fmt.Sprintf("%v\n%v", previous, combined.String()))
	}
}

func (w *win) showError(err error) {
	if err == nil {
		return
	}
	dialog.NewError(err, w.widget).Show()
}

func (w *win) snapshot() {
	fd, err := os.OpenFile("./snapshot.json", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		w.showError(err)
	}
	defer fd.Close()
	err = w.sh.Snapshot(w.ctx, fd)
	if err != nil {
		w.showError(err)
	}
	err = fd.Sync()
	if err != nil {
		dialog.NewError(err, w.widget).Show()
	}
}

func (w *win) saveHistory() {
	w.history.dedup()
	buf, err := json.Marshal(w.history)
	if err != nil {
		w.showError(err)
	}
	err = os.WriteFile("./history.json", buf, 0600)
	if err != nil {
		w.showError(err)
	}
}

func (w *win) loadHistory() {
	buf, err := os.ReadFile("./history.json")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		w.showError(err)
		return
	}
	err = json.Unmarshal(buf, &w.history)
	if err != nil {
		w.showError(err)
		return
	}
	w.history.idx = len(w.history.Entries)
}

func (w *win) reloadSnapshot() {
	fd, err := os.Open("./snapshot.json")
	if err != nil {
		dialog.NewError(err, w.widget).Show()
	}
	defer fd.Close()
	err = w.sh.RestoreSnapshot(w.ctx, fd)
	if err != nil {
		dialog.NewError(err, w.widget).Show()
	}
}

func (w *win) updateHistory(back bool) {
	if back {
		w.nextCmd.Set(w.history.back())
	} else {
		w.nextCmd.Set(w.history.forward())
	}
}

func Run(ctx context.Context, sh Shell) error {
	a := app.New()
	w := a.NewWindow("Appshell")
	w.Resize(fyne.Size{Width: 960, Height: 600})

	win := &win{
		widget:  w,
		output:  binding.NewString(),
		nextCmd: binding.NewString(),

		sh:  sh,
		ctx: ctx,
	}

	outputView := widget.NewEntryWithData(win.output)
	outputView.MultiLine = true
	scroller := binding.NewDataListener(func() {
		outputView.CursorRow = math.MaxInt
		outputView.Refresh()
	})
	runtime.SetFinalizer(outputView, func(_ any) { win.output.RemoveListener(scroller) })
	win.output.AddListener(scroller)

	nextCmdView := newCodeEntryWithData(win.nextCmd)
	nextCmdView.MultiLine = true
	nextCmdView.SetMinRowsVisible(5)

	runBtn := widget.NewButton("Run", func() { win.evalCmd(true) })
	snapshotBtn := widget.NewButton("Snapshot", win.snapshot)
	reloadBtn := widget.NewButton("Reload", win.reloadSnapshot)
	hbox := container.New(hfill{}, nextCmdView, container.NewPadded(container.NewVBox(runBtn, snapshotBtn, reloadBtn)))
	vs := container.NewVSplit(outputView, hbox)
	vs.SetOffset(1.0)

	evalcmd := func(_ fyne.Shortcut) {
		win.evalCmd(true)
	}
	updateHistory := func(back bool) func(_ fyne.Shortcut) {
		return func(_ fyne.Shortcut) {
			win.updateHistory(back)
		}
	}
	nextCmdView.RegisterShortcut(fyne.KeyReturn, fyne.KeyModifierControl, evalcmd)
	nextCmdView.RegisterShortcut(fyne.KeyReturn, fyne.KeyModifierSuper, evalcmd)
	nextCmdView.RegisterShortcut(fyne.KeyUp, fyne.KeyModifierControl, updateHistory(true))
	nextCmdView.RegisterShortcut(fyne.KeyUp, fyne.KeyModifierSuper, updateHistory(true))
	nextCmdView.RegisterShortcut(fyne.KeyDown, fyne.KeyModifierControl, updateHistory(false))
	nextCmdView.RegisterShortcut(fyne.KeyDown, fyne.KeyModifierSuper, updateHistory(false))

	w.SetOnClosed(func() {
		win.saveHistory()
	})

	w.SetContent(vs)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		w.Close()
	}()
	w.Show()

	win.loadHistory()

	w.ShowAndRun()
	return ctx.Err()
}
