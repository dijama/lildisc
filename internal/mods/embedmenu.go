package mods

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/diamondburned/chatkit/components/embed"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
)

// AttachEmbedContextMenu adds a right-click context menu to an embed widget
// with Save As, Copy URL, and Open in Browser options.
func AttachEmbedContextMenu(ctx context.Context, embedWidget *embed.Embed, sourceURL, filename string) {
	if sourceURL == "" {
		return
	}
	if filename == "" {
		filename = filenameFromURL(sourceURL)
	}

	actions := map[string]func(){
		"embedctx.save": func() {
			saveEmbedMedia(ctx, embedWidget, sourceURL, filename)
		},
		"embedctx.copy-url": func() {
			display := gdk.DisplayGetDefault()
			if display != nil {
				display.Clipboard().SetText(sourceURL)
			}
		},
		"embedctx.open-browser": func() {
			app.OpenURI(ctx, sourceURL)
		},
	}

	gtkutil.BindActionMap(embedWidget, actions)
	gtkutil.BindPopoverMenuCustom(embedWidget, gtk.PosBottom, []gtkutil.PopoverMenuItem{
		gtkutil.MenuItemIcon("_Save As…", "embedctx.save", "document-save-symbolic"),
		gtkutil.MenuItemIcon("_Copy URL", "embedctx.copy-url", "edit-copy-symbolic"),
		gtkutil.MenuItemIcon("Open in _Browser", "embedctx.open-browser", "web-browser-symbolic"),
	})
}

func saveEmbedMedia(ctx context.Context, parent gtk.Widgetter, sourceURL, filename string) {
	chooser := gtk.NewFileChooserNative("Save Media", nil, gtk.FileChooserActionSave, "Save", "Cancel")
	chooser.SetCurrentName(filename)

	var signal glib.SignalHandle
	signal = chooser.ConnectResponse(func(resp int) {
		chooser.HandlerDisconnect(signal)
		if resp != int(gtk.ResponseAccept) {
			return
		}

		file := chooser.File()
		outPath := file.Path()

		go func() {
			if err := downloadFile(sourceURL, outPath); err != nil {
				slog.Error("failed to save media", "url", sourceURL, "path", outPath, "err", err)
			} else {
				slog.Info("media saved", "path", outPath)
			}
		}()
	})

	chooser.Show()
}

func downloadFile(url, outPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: unexpected status %d", resp.StatusCode)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func filenameFromURL(u string) string {
	return MediaFilename(u, "")
}

// MediaFilename extracts a usable filename from a URL. If the URL's path
// doesn't contain a recognisable media extension, fallbackExt (e.g. ".mp4")
// is appended. This also fixes URLs like Twitter's ".../1280x720/go" where
// path.Base returns "go" with no extension.
func MediaFilename(u string, fallbackExt string) string {
	// Strip query params.
	if idx := strings.IndexByte(u, '?'); idx >= 0 {
		u = u[:idx]
	}
	name := path.Base(u)
	if name == "" || name == "." || name == "/" {
		name = "media"
	}

	ext := path.Ext(name)
	if isMediaExtension(ext) {
		return name
	}

	// No recognisable extension — append fallback.
	if fallbackExt != "" {
		if !strings.HasPrefix(fallbackExt, ".") {
			fallbackExt = "." + fallbackExt
		}
		return name + fallbackExt
	}
	return name
}

func isMediaExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg",
		".mp4", ".webm", ".mov", ".avi", ".mkv", ".m4v",
		".mp3", ".ogg", ".wav", ".flac":
		return true
	}
	return false
}
