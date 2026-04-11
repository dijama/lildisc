package mods

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var enableMpv = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Use mpv",
	Section:     "Mods",
	Description: "Use mpv for video playback on supported sites. Disable if mpv is not installed.",
})

var enableYtdlp = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Use yt-dlp",
	Section:     "Mods",
	Description: "Use yt-dlp to extract playable URLs from YouTube and other streaming sites for native in-app playback. Disable if yt-dlp is not installed.",
})

// videoHosts are domains where yt-dlp extraction or mpv+yt-dlp should be used.
var videoHosts = []string{
	"youtube.com",
	"youtu.be",
	"www.youtube.com",
	"m.youtube.com",
	"twitch.tv",
	"www.twitch.tv",
	"clips.twitch.tv",
	"streamable.com",
	"www.streamable.com",
	"vimeo.com",
	"www.vimeo.com",
}

// TryPlayVideo attempts to play a URL with mpv. Returns true if handled,
// false if the caller should fall back.
func TryPlayVideo(url string) bool {
	if !enableMpv.Value() {
		return false
	}

	if !IsVideoHost(url) {
		return false
	}

	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		slog.Debug("mpv not found, falling back")
		return false
	}

	slog.Info("playing video with mpv", "url", url)
	cmd := exec.Command(mpvPath,
		"--force-window=immediate",
		"--ytdl-path=yt-dlp",
		"--title=LilDisc Video",
		"--no-terminal",
		// GPU acceleration + Wayland native
		"--hwdec=auto",
		"--gpu-context=wayland",
		"--vo=gpu-next",
		// Streaming buffer for network sources
		"--cache=yes",
		"--demuxer-max-bytes=50MiB",
		"--demuxer-max-back-bytes=25MiB",
		// Cap quality to avoid huge downloads
		"--ytdl-raw-options=format=bestvideo[height<=1080]+bestaudio/best[height<=1080]",
		url,
	)
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to start mpv", "err", err)
		return false
	}

	// Don't wait — let mpv run independently.
	go cmd.Wait()
	return true
}

// ExtractStreamURL uses yt-dlp to get a direct playable URL from a streaming
// site. Blocks until extraction completes — call from a goroutine.
func ExtractStreamURL(url string) (string, error) {
	if !enableYtdlp.Value() {
		return "", fmt.Errorf("yt-dlp disabled")
	}

	ytdlpPath, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	slog.Info("extracting stream URL", "url", url)
	cmd := exec.CommandContext(ctx, ytdlpPath,
		"--get-url",
		"-f", "best[height<=1080]/best",
		url,
	)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("yt-dlp: %w", err)
	}

	// yt-dlp may output multiple lines (video + audio); take the first.
	directURL := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if directURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty URL")
	}

	slog.Info("extracted stream URL", "direct", directURL[:min(len(directURL), 80)])
	return directURL, nil
}

// IsVideoHost reports whether the URL belongs to a streaming host where
// yt-dlp should be used.
func IsVideoHost(url string) bool {
	lower := strings.ToLower(url)
	for _, host := range videoHosts {
		if strings.Contains(lower, host) {
			return true
		}
	}
	return false
}

// VideoLoadingWindow is a small popup with a spinner shown while yt-dlp
// extracts a stream URL.
type VideoLoadingWindow struct {
	*adw.Window
	spinner *gtk.Spinner
	label   *gtk.Label
}

var videoLoadingCSS = cssutil.Applier("video-loading-window", `
	.video-loading-window {
		background: alpha(@theme_bg_color, 0.95);
	}
	.video-loading-box {
		padding: 24px 32px;
	}
	.video-loading-label {
		margin-top: 12px;
		font-size: 1.1em;
		opacity: 0.7;
	}
`)

// NewVideoLoadingWindow creates and shows a loading popup.
func NewVideoLoadingWindow(ctx context.Context) *VideoLoadingWindow {
	w := &VideoLoadingWindow{}

	w.spinner = gtk.NewSpinner()
	w.spinner.SetSizeRequest(48, 48)
	w.spinner.Start()

	w.label = gtk.NewLabel("Loading video...")
	w.label.AddCSSClass("video-loading-label")

	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.AddCSSClass("video-loading-box")
	box.SetVAlign(gtk.AlignCenter)
	box.SetHAlign(gtk.AlignCenter)
	box.Append(w.spinner)
	box.Append(w.label)

	header := adw.NewHeaderBar()
	header.SetShowStartTitleButtons(true)
	header.SetShowEndTitleButtons(true)

	content := gtk.NewBox(gtk.OrientationVertical, 0)
	content.Append(header)
	content.Append(box)
	box.SetVExpand(true)

	w.Window = adw.NewWindow()
	w.SetTitle("LilDisc Video")
	w.SetDefaultSize(360, 200)
	w.SetModal(false)
	w.SetContent(content)

	parent := app.GTKWindowFromContext(ctx)
	if parent != nil {
		w.SetTransientFor(parent)
	}

	videoLoadingCSS(w)
	w.Present()

	return w
}

// SetError updates the label to show an error state and stops the spinner.
func (w *VideoLoadingWindow) SetError(msg string) {
	w.spinner.Stop()
	w.label.SetText(msg)
}
