package gui

import (
	"fyne.io/fyne/v2"
)

type (
	// hfill layout will expand the first element horizontally
	// and all others will keep their MinSize
	//
	// all elements have the same Y size
	//
	// there is no space between elements
	hfill struct{}
)

func (h hfill) Layout(objs []fyne.CanvasObject, sz fyne.Size) {
	if len(objs) == 1 {
		objs[0].Resize(sz)
		objs[0].Move(fyne.NewPos(0, 0))
		return
	}

	tailSize := h.MinSize(objs[1:])
	first := fyne.NewSize(sz.Width-tailSize.Width, sz.Height)
	objs[0].Resize(first)
	objs[0].Move(fyne.NewPos(0, 0))

	leftMargin := float32(first.Width)
	for _, rest := range objs[1:] {
		mz := rest.MinSize()
		mz.Height = sz.Height
		rest.Resize(mz)
		rest.Move(fyne.NewPos(leftMargin, 0))
		leftMargin += mz.Width
	}
}

func (h hfill) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objects {
		sz := o.MinSize()
		min.Width += sz.Width
		if min.Height < sz.Height {
			min.Height = sz.Height
		}
	}
	return min
}
