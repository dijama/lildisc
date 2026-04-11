package main

import (
	"context"
	"embed"
	"io/fs"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/locale"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/dijama/lildisc/internal/gtkcord"
	"github.com/dijama/lildisc/internal/mods"
	"github.com/dijama/lildisc/internal/window"
	"github.com/dijama/lildisc/internal/window/about"

	_ "github.com/diamondburned/gotkit/gtkutil/aggressivegc"
	_ "github.com/dijama/lildisc/internal/icons"
)

//go:embed po/*
var po embed.FS

func init() {
	po, _ := fs.Sub(po, "po")
	locale.LoadLocale(po)
}

// Version is connected to about.SetVersion.
var Version string

func init() { about.SetVersion(Version) }

var _ = cssutil.WriteCSS(`
	window.background,
	window.background.solid-csd {
		background-color: @theme_bg_color;
	}

	avatar > image {
		background: none;
	}
	avatar > label {
		background: @borders;
	}

	.md-textblock {
		line-height: 1.35em;
	}

	/* Blockquotes: tighter line spacing than regular text.
	   The 1.35em base line-height is too airy inside > quotes. */
	.md-blockquote .md-textblock {
		line-height: 1.15em;
	}

	/* Code blocks: reduce excess bottom padding inside the scroll frame.
	   Upstream has padding: 4px 6px on the text; the scroll propagation
	   adds extra dead space below short blocks. */
	.md-codeblock-frame textview {
		padding: 4px 6px 2px 6px;
	}
	.md-codeblock-frame scrolledwindow {
		margin-bottom: 0;
		padding-bottom: 0;
	}

	/* Reply previews: give the content a bit more breathing room.
	   The 0.9em font + tight blockquote padding makes replies feel cramped. */
	.message-reply-box {
		padding-top: 2px;
		padding-bottom: 4px;
	}
	.message-reply-box .mauthor-chip {
		margin-bottom: 1px;
	}
	.message-reply-content {
		margin-top: 1px;
	}
`)

func init() {
	app.Hook(func(*app.Application) {
		adw.Init()
		adaptive.Init()
	})
}

func main() {
	m := manager{}
	m.app = app.New(context.Background(), "io.github.dijama.lildisc", "LilDisc")
	m.app.AddJSONActions(map[string]interface{}{
		"app.preferences": func() { prefui.ShowDialog(m.win.Context()) },
		"app.about":       func() { about.New(m.win.Context()).Present(m.win) },
		"app.logs":        func() { logui.ShowDefaultViewer(m.win.Context()) },
		"app.quit":        func() { m.app.Quit() },
	})
	m.app.AddActionCallbacks(map[string]gtkutil.ActionCallback{
		"app.open-channel": m.forwardSignalToWindow("open-channel", gtkcord.SnowflakeVariant),
		"app.open-guild":   m.forwardSignalToWindow("open-guild", gtkcord.SnowflakeVariant),
	})
	m.app.AddActionShortcuts(map[string]string{
		"<Ctrl>Q": "app.quit",
	})
	m.app.ConnectActivate(func() { m.activate(m.app.Context()) })
	m.app.RunMain()
}

type manager struct {
	app *app.Application
	win *window.Window
}

func (m *manager) forwardSignalToWindow(name string, t *glib.VariantType) gtkutil.ActionCallback {
	return gtkutil.ActionCallback{
		ArgType: t,
		Func:    func(args *glib.Variant) { m.win.ActivateAction(name, args) },
	}
}

func (m *manager) activate(ctx context.Context) {
	if m.win != nil {
		m.win.Present()
		return
	}

	m.win = window.NewWindow(ctx)
	m.win.Present()

	// mod: initialize all mods after window is ready
	mods.Init(ctx, m.win)

	prefs.AsyncLoadSaved(ctx, func(err error) {
		if err != nil {
			app.Error(ctx, err)
		}
	})
}
