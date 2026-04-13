# LilDisc

A native GTK4/Go Discord client for Linux. Wayland-first, lightweight, with a modular feature system inspired by Vesktop.

LilDisc is a standalone application — not a wrapper around Discord's web UI. It speaks directly to Discord's gateway via [arikawa](https://github.com/diamondburned/arikawa) and renders everything natively in GTK4 / libadwaita.

## Features

All optional features are independently toggleable in the Mods preferences pane and live as self-contained modules under `internal/mods/`.

### Embed & Media
- **Auto-animate GIFs** inline instead of requiring click
- **Full-size Twitter/X images** — renders images at proper size inside embed cards, not as tiny thumbnails
- **Direct URL loading** — bypasses Discord's broken external proxy for fixupx, fxtwitter, vxtwitter and similar services
- **Multi-image grid** — Twitter multi-image posts render in a 2-column grid inside the embed card
- **Prefer GIF over MP4** — loads actual .gif for Giphy embeds instead of video
- **Full-width embed cards** — rich embeds fill the message area
- **Content-sized viewer** — image/video popup matches media dimensions

### Right-Click on Media
- **Save As** — download source file with proper filename
- **Copy URL** — copy original source URL
- **Open in Browser** — open in default browser

### Sidebar
- **Resizable sidebar** — draggable divider between sidebar and chat
- **Compact mode** — narrows below 180px, collapses to avatar-only
- **Presence badges** — colored status dots overlaid on avatar corners (online/idle/DND/offline)
- **Friend nicknames** — personal nicknames set via Discord Relationships are used everywhere (DM list, group DM names, chat author labels, quick switcher)
- **Friends dropdown in DMs** — collapsible "Friends (N)" section under the active DM list showing friends who don't currently have an open DM; click to start a new conversation

### Composer
- **Drag-and-drop upload** — drop files onto the message input
- **Clipboard image paste** — Ctrl+V works for images copied from browsers
- **Reply preview bar** — shows what you're replying to above the input
- **GIF picker** — Tenor search with inline previews
- **Sticker picker** — server stickers and default sticker packs, grouped by source
- **Oversize upload fallback** — files too large for Discord's attachment limit are uploaded to [litterbox.catbox.moe](https://litterbox.catbox.moe) (72h expiry, 1 GiB cap) with [0x0.st](https://0x0.st) as a second fallback, and the resulting URL is pasted into the composer in place of the attachment

### Channels
- **Channel context menu** — right-click for mark-as-read and copy-channel-ID

### Other
- **Custom CSS** — loads `~/.config/lildisc/custom.css`
- **System tray** — minimize to tray on close
- **Message search** — Ctrl+F within channels
- **Keyboard shortcuts** — Ctrl+/ for help overlay
- **Avatar cache refresh** — clears stale avatars when profiles change
- **Server emoji picker** — all server emojis with fuzzy search
- **Notification controls** — notify-all and mute-respect options
- **mpv video player** — plays videos via mpv instead of the built-in player
- **Cache auto-clean** — periodically clears stale image cache entries
- **Lazy load embeds** — defers loading embed images until they scroll into view

## Building

```bash
# Prerequisites (Arch/Manjaro)
sudo pacman -S go gtk4 gobject-introspection

# Build
go build -v -o lildisc .

# Run
./lildisc
```

First build takes ~20 minutes (CGo/GTK4 bindings). Incremental builds are fast.

Requires Go 1.24+.

## Architecture

All features are in `internal/mods/` as independent files. Each mod has a preference toggle and hooks into the host app via minimal integration points marked with `// mod: feature-name` comments.

```
internal/mods/
  mods.go            Init()/HookState() entry points
  apicache.go        Shared on-disk JSON cache helper
  avatarcache.go     Avatar cache busting + manual-refresh signaler
  cacheclean.go      Cache auto-clean
  channelmenu.go     Channel context menu
  compactsidebar.go  Compact avatar-only sidebar mode
  customcss.go       Custom CSS loading
  dragdrop.go        Drag-and-drop file upload
  embedmenu.go       Right-click save/copy/open on media
  embeds.go          GIF/video autoplay, GIFV-to-GIF, embed improvements
  emojipicker.go     Server emoji picker
  friendlist.go      Collapsible "Friends" dropdown in DM sidebar
  gifpicker.go       Tenor-backed GIF picker
  keybinds.go        Keyboard shortcuts
  lazyload.go        Lazy load embeds
  mediahost.go       Oversize-upload fallback (litterbox / 0x0.st)
  notifications.go   Notification improvements
  presence.go        Status indicator dots
  replypreview.go    Reply preview bar
  search.go          Message search
  stickerpicker.go   Sticker picker
  tray.go            System tray
  videoplayer.go     mpv video player
```

## Logging In

Use your Discord user token:

1. Open Discord web app, press F12 for Inspector
2. Go to Network tab, press F5 to refresh
3. Filter for `discord api`, click any request
4. Copy the `Authorization` header value
5. Paste into the Token field in LilDisc

**IMPORTANT:** Using an unofficial client is against Discord's Terms of Service
and may cause your account to be banned! While LilDisc tries its best to not
use the REST API at all unless necessary to reduce the risk of abuse, it is
still possible that Discord may ban your account for using it. Please use
LilDisc at your own risk!

## Credits

LilDisc began as a fork of [Dissent](https://github.com/diamondburned/dissent) by diamondburned, and still shares its rendering core.

- [Dissent](https://github.com/diamondburned/dissent) — the original native GTK4 Discord client this project descends from
- [Vesktop](https://github.com/Vencord/Vesktop) — design inspiration for the modular feature system
- [arikawa](https://github.com/diamondburned/arikawa), [gotk4](https://github.com/diamondburned/gotk4), [gotkit](https://github.com/diamondburned/gotkit), [chatkit](https://github.com/diamondburned/chatkit), [ningen](https://github.com/diamondburned/ningen) — core libraries by diamondburned

## License

GPL-3.0.
