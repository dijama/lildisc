package mods

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var enableCompactSidebar = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Compact DM Sidebar",
	Section:     "Mods",
	Description: "When the sidebar is narrow, show only avatars without names.",
})

var _ = cssutil.WriteCSS(`
	.mod-sidebar-compact .direct-channel {
		padding: 4px 2px;
	}
	.mod-sidebar-compact .direct-channel-avatar {
		margin-right: 0;
	}
`)

// compactThreshold is the width (in px) of the sidebar's Right section
// below which we collapse to icon-only mode. At ~180px, usernames are
// too squished to be useful so we hide them.
const compactThreshold = 180

// SetupCompactSidebar watches a sidebar widget's width and toggles
// icon-only mode when narrow by hiding name labels programmatically.
func SetupCompactSidebar(widget gtk.Widgetter) {
	if !enableCompactSidebar.Value() {
		return
	}

	base := gtk.BaseWidget(widget)
	isCompact := false

	base.AddTickCallback(func(widget gtk.Widgetter, _ gdk.FrameClocker) bool {
		w := gtk.BaseWidget(widget)
		width := w.AllocatedWidth()
		if width <= 0 {
			return true
		}

		shouldBeCompact := width < compactThreshold
		if shouldBeCompact == isCompact {
			return true
		}
		isCompact = shouldBeCompact

		if isCompact {
			w.AddCSSClass("mod-sidebar-compact")
		} else {
			w.RemoveCSSClass("mod-sidebar-compact")
		}

		// Walk the widget tree and hide/show name labels and search bars.
		setCompactChildren(w, isCompact)
		return true
	})
}

func setCompactChildren(widget *gtk.Widget, compact bool) {
	childW := widget.FirstChild()
	for childW != nil {
		child := gtk.BaseWidget(childW)
		classes := child.CSSClasses()
		for _, c := range classes {
			switch c {
			case "direct-channel-name", "direct-channel-readindicator", "direct-searchbar",
				"user-bar-name", "user-bar-menu", "user-bar-status":
				child.SetVisible(!compact)
			case "direct-channel", "user-bar":
				// Center the avatar when compact by centering the row's box.
				if compact {
					child.SetHAlign(gtk.AlignCenter)
				} else {
					child.SetHAlign(gtk.AlignFill)
				}
			}
		}
		setCompactChildren(child, compact)
		childW = child.NextSibling()
	}
}
