package sidebar

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/dijama/lildisc/internal/gtkcord"
	"github.com/dijama/lildisc/internal/mods"
)

type userBar struct {
	// *gtk.ActionBar
	*gtk.Box
	avatar        *onlineimage.Avatar
	avatarOverlay *gtk.Overlay // mod: presence dot overlay
	name          *gtk.Label
	menu          *gtk.ToggleButton

	ctx context.Context
}

var userBarCSS = cssutil.Applier("user-bar", `
	.user-bar-avatar {
		padding: 6px;
	}
	.user-bar-menu {
		margin: 0 6px;
	}
`)

func newUserBar(ctx context.Context, menuActions []gtkutil.PopoverMenuItem) *userBar {
	b := userBar{ctx: ctx}
	b.avatar = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.UserBarAvatarSize)
	b.avatar.AddCSSClass("user-bar-avatar")

	// mod: presence — overlay status dot on avatar, same as DM channel avatars
	b.avatarOverlay = gtk.NewOverlay()
	b.avatarOverlay.SetChild(b.avatar)

	b.name = gtk.NewLabel("")
	b.name.AddCSSClass("user-bar-name")
	b.name.SetSelectable(true)
	b.name.SetXAlign(0)
	b.name.SetHExpand(true)
	b.name.SetWrap(false)
	b.name.SetEllipsize(pango.EllipsizeEnd)

	b.menu = gtk.NewToggleButton()
	b.menu.AddCSSClass("user-bar-menu")
	b.menu.SetIconName("menu-large-symbolic")
	b.menu.SetTooltipText("Main Menu")
	b.menu.SetHasFrame(false)
	b.menu.SetVAlign(gtk.AlignCenter)
	b.menu.ConnectClicked(func() {
		p := gtkutil.NewPopoverMenuCustom(b.menu, gtk.PosTop, menuActions)
		p.ConnectHide(func() { b.menu.SetActive(false) })
		gtkutil.PopupFinally(p)
	})

	// mod: compactsidebar — clicking the avatar opens the menu,
	// so the menu is accessible when the menu button is hidden.
	avatarClick := gtk.NewGestureClick()
	avatarClick.ConnectReleased(func(n int, x, y float64) {
		p := gtkutil.NewPopoverMenuCustom(b.avatarOverlay, gtk.PosTop, menuActions)
		gtkutil.PopupFinally(p)
	})
	b.avatarOverlay.AddController(avatarClick)

	b.Box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	b.Box.Append(b.avatarOverlay)
	b.Box.Append(b.name)
	b.Box.Append(b.menu)
	userBarCSS(b)

	anim := b.avatar.EnableAnimation()
	anim.ConnectMotion(b)

	vis := gtkutil.WithVisibility(ctx, b)

	client := gtkcord.FromContext(ctx)
	client.BindHandler(vis,
		func(ev gateway.Event) {
			switch ev := ev.(type) {
			case *gateway.UserUpdateEvent:
				b.updateUser(&ev.User)
			case *gateway.ReadyEvent:
				// ReadyEvent has the full user with avatar hash.
				b.updateUser(&ev.User)
			}
		},
		(*gateway.UserUpdateEvent)(nil),
		(*gateway.ReadyEvent)(nil),
	)

	me, _ := client.Me()
	if me != nil {
		b.updateUser(me)

		// mod: presence — status dot on avatar, same as DM channel list
		dot := mods.NewPresenceDot(ctx, me.ID, 0)
		if dot != nil {
			dot.SetHAlign(gtk.AlignEnd)
			dot.SetVAlign(gtk.AlignEnd)
			b.avatarOverlay.AddOverlay(dot)
		}
	}

	// mod: avatar — fetch and save avatar to local file if not cached.
	// Bypasses gotkit's image cache entirely to avoid stale/404 issues.
	if mods.LocalAvatarPath() == "" && me != nil && me.Avatar != "" {
		token := client.Token()
		meID := me.ID.String()
		meAvatar := me.Avatar
		go func() {
			mods.FetchAndSaveAvatar(token, meID, meAvatar)
			glib.IdleAdd(func() { b.updateUser(me) })
		}()
	} else if me != nil && me.Avatar == "" {
		go func() {
			apiMe := client.FetchMeFromAPI()
			if apiMe != nil && apiMe.Avatar != "" {
				token := client.Token()
				mods.FetchAndSaveAvatar(token, apiMe.ID.String(), apiMe.Avatar)
				glib.IdleAdd(func() { b.updateUser(apiMe) })
			}
		}()
	}

	return &b
}

var discriminatorRe = regexp.MustCompile(`#\d{1,4}$`)

func (b *userBar) updateUser(me *discord.User) {
	slog.Info("userBar.updateUser",
		"id", me.ID,
		"username", me.Username,
		"avatar", me.Avatar,
		"url", gtkcord.InjectAvatarSize(me.AvatarURL()))
	tag := me.Username
	if v, _ := strconv.Atoi(me.Discriminator); v != 0 {
		tag += `<span size="smaller">` + "#" + me.Discriminator + "</span>"
	}

	var name string
	if me.DisplayName != "" {
		name = me.DisplayName + "\n" + `<span size="smaller">` + tag + "</span>"
	} else {
		name = tag
	}

	displayName := me.DisplayName
	if displayName == "" {
		displayName = me.Username
	}

	b.avatar.SetText(displayName)
	// mod: avatar — load from local cached file, bypassing gotkit's
	// image cache entirely. See ensureLocalAvatar().
	if path := mods.LocalAvatarPath(); path != "" {
		b.avatar.SetFromURL("file://" + path)
	}
	b.name.SetMarkup(name)
	b.name.SetTooltipMarkup(name)
}

