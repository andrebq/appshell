package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type (
	codeEntry struct {
		widget.Entry
		shortcuts fyne.ShortcutHandler
	}
)

func newCodeEntryWithData(data binding.String) *codeEntry {
	ce := &codeEntry{}
	ce.ExtendBaseWidget(ce)
	ce.Bind(data)
	return ce
}

func (ce *codeEntry) RegisterShortcut(keyName fyne.KeyName, mod fyne.KeyModifier, handler func(fyne.Shortcut)) {
	ce.shortcuts.AddShortcut(&desktop.CustomShortcut{
		KeyName:  keyName,
		Modifier: mod,
	}, handler)
}

func (ce *codeEntry) TypedShortcut(s fyne.Shortcut) {
	if _, ok := s.(*desktop.CustomShortcut); !ok {
		ce.Entry.TypedShortcut(s)
		return
	}
	ce.shortcuts.TypedShortcut(s)
}
