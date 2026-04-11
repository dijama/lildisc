package mods

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

var enableTray = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Close to Tray",
	Section:     "Mods",
	Description: "Minimize to system tray when closing the window instead of quitting.",
})

const (
	sniDBusPath  = "/StatusNotifierItem"
	sniItemIface = "org.freedesktop.StatusNotifierItem"
	menuDBusPath = "/MenuBar"
	menuIface    = "com.canonical.dbusmenu"
)

// trayItem holds the D-Bus connection and window reference.
type trayItem struct {
	conn *dbus.Conn
	win  gtk.Widgetter
}

func initTray(ctx context.Context, win ActionWidget) {
	if !enableTray.Value() {
		return
	}

	// Hold the application so it stays alive when the window is hidden.
	app.FromContext(ctx).Hold()

	// Intercept window close: hide instead of destroy.
	base := gtk.BaseWidget(win)
	var connected bool
	base.ConnectMap(func() {
		if connected {
			return
		}
		connected = true
		gtkWin := base.Root().CastType(gtk.GTypeWindow).(*gtk.Window)
		gtkWin.ConnectCloseRequest(func() (stop bool) {
			if !enableTray.Value() {
				return false
			}
			gtkWin.SetVisible(false)
			return true
		})
	})

	// Start the SNI tray icon.
	go func() {
		if err := startSNI(win); err != nil {
			slog.Warn("failed to start system tray icon", "err", err)
			slog.Info("close-to-tray will still hide the window, but no tray icon will appear")
		}
	}()
}

func startSNI(win gtk.Widgetter) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect to session bus: %w", err)
	}

	item := &trayItem{conn: conn, win: win}
	menu := &trayMenu{item: item}
	busName := conn.Names()[0]

	// Export SNI on both freedesktop and KDE interfaces.
	conn.Export(item, sniDBusPath, sniItemIface)
	conn.Export(item, sniDBusPath, "org.kde.StatusNotifierItem")
	conn.Export(item, sniDBusPath, "org.freedesktop.DBus.Properties")
	conn.Export(introspect.NewIntrospectable(&introspect.Node{
		Name: sniDBusPath,
		Interfaces: []introspect.Interface{{
			Name: sniItemIface,
			Methods: []introspect.Method{
				{Name: "Activate", Args: []introspect.Arg{
					{Name: "x", Type: "i", Direction: "in"},
					{Name: "y", Type: "i", Direction: "in"},
				}},
				{Name: "ContextMenu", Args: []introspect.Arg{
					{Name: "x", Type: "i", Direction: "in"},
					{Name: "y", Type: "i", Direction: "in"},
				}},
			},
			Properties: []introspect.Property{
				{Name: "Category", Type: "s", Access: "read"},
				{Name: "Id", Type: "s", Access: "read"},
				{Name: "Title", Type: "s", Access: "read"},
				{Name: "Status", Type: "s", Access: "read"},
				{Name: "IconName", Type: "s", Access: "read"},
				{Name: "Menu", Type: "o", Access: "read"},
				{Name: "ItemIsMenu", Type: "b", Access: "read"},
			},
		}},
	}), sniDBusPath, "org.freedesktop.DBus.Introspectable")

	// Export the dbusmenu for right-click context menu.
	conn.Export(menu, menuDBusPath, menuIface)
	conn.Export(menu, menuDBusPath, "org.freedesktop.DBus.Properties")

	// Register with watcher (KDE first, then freedesktop).
	var registered bool
	for _, watcher := range []string{"org.kde.StatusNotifierWatcher", "org.freedesktop.StatusNotifierWatcher"} {
		obj := conn.Object(watcher, "/StatusNotifierWatcher")
		call := obj.Call(watcher+".RegisterStatusNotifierItem", 0, busName)
		if call.Err == nil {
			registered = true
			slog.Info("registered tray icon", "watcher", watcher)
			break
		}
	}
	if !registered {
		return fmt.Errorf("no StatusNotifierWatcher found")
	}

	slog.Info("system tray icon registered")
	select {} // block forever
}

// --- SNI Item methods ---

func (t *trayItem) Activate(x, y int32) *dbus.Error {
	glib.IdleAdd(func() {
		base := gtk.BaseWidget(t.win)
		gtkWin := base.Root().CastType(gtk.GTypeWindow).(*gtk.Window)
		if gtkWin.IsVisible() {
			gtkWin.SetVisible(false)
		} else {
			gtkWin.SetVisible(true)
			gtkWin.Present()
		}
	})
	return nil
}

func (t *trayItem) ContextMenu(x, y int32) *dbus.Error {
	// Waybar's dbusmenu popup positioning is broken on bottom bars,
	// so handle quit directly via a GTK popover on the window instead.
	glib.IdleAdd(func() {
		base := gtk.BaseWidget(t.win)
		gtkWin := base.Root().CastType(gtk.GTypeWindow).(*gtk.Window)

		// If window is hidden, show it first so the user can interact.
		if !gtkWin.IsVisible() {
			gtkWin.SetVisible(true)
			gtkWin.Present()
		}

		// Show a simple dialog asking to quit.
		dialog := gtk.NewMessageDialog(
			gtkWin,
			gtk.DialogModal|gtk.DialogDestroyWithParent,
			gtk.MessageQuestion,
			gtk.ButtonsYesNo,
		)
		dialog.SetMarkup("Quit LilDisc?")
		dialog.ConnectResponse(func(resp int) {
			dialog.Destroy()
			if resp == int(gtk.ResponseYes) {
				os.Exit(0)
			}
		})
		dialog.Show()
	})
	return nil
}

// --- SNI Properties ---

func (t *trayItem) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if iface != sniItemIface && iface != "org.kde.StatusNotifierItem" {
		return dbus.Variant{}, nil
	}
	props := sniProperties()
	if v, ok := props[prop]; ok {
		return v, nil
	}
	return dbus.Variant{}, nil
}

func (t *trayItem) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != sniItemIface && iface != "org.kde.StatusNotifierItem" {
		return nil, nil
	}
	return sniProperties(), nil
}

func (t *trayItem) Set(iface, prop string, value dbus.Variant) *dbus.Error {
	return nil
}

func sniProperties() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Category":      dbus.MakeVariant("Communications"),
		"Id":            dbus.MakeVariant("lildisc"),
		"Title":         dbus.MakeVariant("LilDisc"),
		"Status":        dbus.MakeVariant("Active"),
		"IconName":      dbus.MakeVariant("discord-tray"),
		"IconThemePath": dbus.MakeVariant(""),
		"ItemIsMenu":    dbus.MakeVariant(false),
		"WindowId":      dbus.MakeVariant(int32(0)),
		"Menu":          dbus.MakeVariant(dbus.ObjectPath(menuDBusPath)),
		"IconPixmap": dbus.MakeVariant([]struct {
			W, H int32
			Data []byte
		}{}),
	}
}

// --- DBusMenu for right-click context ---

// Menu item IDs
const (
	menuIDRoot = 0
	menuIDShow = 1
	menuIDQuit = 2
)

type trayMenu struct {
	item   *trayItem
	revision uint32
}

// GetLayout implements com.canonical.dbusmenu.
func (m *trayMenu) GetLayout(parentID int32, recursionDepth int32, propertyNames []string) (uint32, struct {
	V0 int32
	V1 map[string]dbus.Variant
	V2 []dbus.Variant
}, *dbus.Error) {
	type menuItem struct {
		V0 int32
		V1 map[string]dbus.Variant
		V2 []dbus.Variant
	}

	show := menuItem{
		V0: menuIDShow,
		V1: map[string]dbus.Variant{
			"label":   dbus.MakeVariant("Show/Hide"),
			"enabled": dbus.MakeVariant(true),
			"visible": dbus.MakeVariant(true),
		},
		V2: []dbus.Variant{},
	}

	quit := menuItem{
		V0: menuIDQuit,
		V1: map[string]dbus.Variant{
			"label":   dbus.MakeVariant("Quit"),
			"enabled": dbus.MakeVariant(true),
			"visible": dbus.MakeVariant(true),
		},
		V2: []dbus.Variant{},
	}

	root := struct {
		V0 int32
		V1 map[string]dbus.Variant
		V2 []dbus.Variant
	}{
		V0: menuIDRoot,
		V1: map[string]dbus.Variant{
			"children-display": dbus.MakeVariant("submenu"),
		},
		V2: []dbus.Variant{
			dbus.MakeVariant(show),
			dbus.MakeVariant(quit),
		},
	}

	return m.revision, root, nil
}

// GetGroupProperties implements com.canonical.dbusmenu.
func (m *trayMenu) GetGroupProperties(ids []int32, propertyNames []string) ([]struct {
	V0 int32
	V1 map[string]dbus.Variant
}, *dbus.Error) {
	return nil, nil
}

// GetProperty implements com.canonical.dbusmenu.
func (m *trayMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	return dbus.Variant{}, nil
}

// Event implements com.canonical.dbusmenu.
func (m *trayMenu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	switch id {
	case menuIDShow:
		m.item.Activate(0, 0)
	case menuIDQuit:
		glib.IdleAdd(func() {
			os.Exit(0)
		})
	}
	return nil
}

// EventGroup implements com.canonical.dbusmenu.
func (m *trayMenu) EventGroup(events []struct {
	V0 int32
	V1 string
	V2 dbus.Variant
	V3 uint32
}) ([]int32, *dbus.Error) {
	for _, ev := range events {
		m.Event(ev.V0, ev.V1, ev.V2, ev.V3)
	}
	return nil, nil
}

// AboutToShow implements com.canonical.dbusmenu.
func (m *trayMenu) AboutToShow(id int32) (bool, *dbus.Error) {
	return false, nil
}

// AboutToShowGroup implements com.canonical.dbusmenu.
func (m *trayMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return nil, nil, nil
}

// --- Menu Properties interface ---

func (m *trayMenu) Get(iface, prop string) (dbus.Variant, *dbus.Error) {
	if prop == "Version" {
		return dbus.MakeVariant(uint32(3)), nil
	}
	if prop == "TextDirection" {
		return dbus.MakeVariant("ltr"), nil
	}
	if prop == "Status" {
		return dbus.MakeVariant("normal"), nil
	}
	if prop == "IconThemePath" {
		return dbus.MakeVariant([]string{}), nil
	}
	return dbus.Variant{}, nil
}

func (m *trayMenu) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return map[string]dbus.Variant{
		"Version":       dbus.MakeVariant(uint32(3)),
		"TextDirection": dbus.MakeVariant("ltr"),
		"Status":        dbus.MakeVariant("normal"),
		"IconThemePath": dbus.MakeVariant([]string{}),
	}, nil
}

func (m *trayMenu) Set(iface, prop string, value dbus.Variant) *dbus.Error {
	return nil
}
