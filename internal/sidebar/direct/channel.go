package direct

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/components/onlineimage"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/gotkit/gtkutil/imgutil"
	"github.com/diamondburned/ningen/v3"
	"github.com/dijama/lildisc/internal/gtkcord"
	"github.com/dijama/lildisc/internal/mods"
)

// Channel is an individual direct messaging channel.
type Channel struct {
	*gtk.ListBoxRow
	box            *gtk.Box
	avatar         *onlineimage.Avatar
	avatarOverlay  *gtk.Overlay // mod: presence — overlay for status badge
	name           *gtk.Label
	readIndicator  *gtk.Label
	presenceDot    *gtk.Label // mod: presence

	ctx context.Context
	id  discord.ChannelID
}

var channelCSS = cssutil.Applier("direct-channel", `
	.direct-channel {
		padding: 4px 6px;
	}
	.direct-channel-avatar {
		margin-right: 0;
	}
`)

// mod: presence — avatar overlay needs the margin instead of the avatar itself
var _ = cssutil.WriteCSS(`
	.direct-channel overlay {
		margin-right: 6px;
	}
	.mod-sidebar-compact .direct-channel overlay {
		margin-right: 0;
	}
`)

// NewChannel creates a new Channel.
func NewChannel(ctx context.Context, id discord.ChannelID) *Channel {
	ch := Channel{
		ctx: ctx,
		id:  id,
	}

	ch.name = gtk.NewLabel("")
	ch.name.AddCSSClass("direct-channel-name")
	ch.name.SetXAlign(0)
	ch.name.SetHExpand(true)
	ch.name.SetEllipsize(pango.EllipsizeEnd)
	ch.name.SetSingleLineMode(true)

	ch.avatar = onlineimage.NewAvatar(ctx, imgutil.HTTPProvider, gtkcord.ChannelIconSize)
	ch.avatar.AddCSSClass("direct-channel-avatar")

	// mod: presence — wrap avatar in overlay for status badge
	ch.avatarOverlay = gtk.NewOverlay()
	ch.avatarOverlay.SetChild(ch.avatar)

	ch.readIndicator = gtk.NewLabel("")
	ch.readIndicator.AddCSSClass("direct-channel-readindicator")

	ch.box = gtk.NewBox(gtk.OrientationHorizontal, 0)
	ch.box.SetHExpand(true) // mod: ensure box fills row so names can expand
	ch.box.Append(ch.avatarOverlay)
	ch.box.Append(ch.name)
	ch.box.Append(ch.readIndicator)

	ch.ListBoxRow = gtk.NewListBoxRow()
	ch.SetChild(ch.box)
	ch.SetName(id.String())
	channelCSS(ch)

	// mod: channelmenu — right-click context menu on DM channels
	mods.AttachChannelContextMenu(ctx, ch.box, id)

	return &ch
}

// Invalidate fetches the same channel from the state and updates itself.
func (ch *Channel) Invalidate() {
	state := gtkcord.FromContext(ch.ctx)

	channel, err := state.Cabinet.Channel(ch.id)
	if err != nil {
		slog.Error(
			"Failed to fetch direct channel from state",
			"channel_id", ch.id,
			"err", err)
		return
	}

	ch.Update(channel)
}

// Update updates the channel to show information from the instance given. ID is
// not checked.
func (ch *Channel) Update(channel *discord.Channel) {
	name := gtkcord.ChannelName(channel)
	ch.name.SetText(name)

	if channel.Type == discord.DirectMessage && len(channel.DMRecipients) > 0 {
		u := channel.DMRecipients[0]
		ch.avatar.SetText(name)
		ch.avatar.SetFromURL(gtkcord.InjectAvatarSize(u.AvatarURL()))

		// mod: presence — status badge overlaid on avatar bottom-right
		if ch.presenceDot == nil {
			dot := mods.NewPresenceDot(ch.ctx, u.ID, 0)
			if dot != nil {
				ch.presenceDot = dot
				dot.SetHAlign(gtk.AlignEnd)
				dot.SetVAlign(gtk.AlignEnd)
				ch.avatarOverlay.AddOverlay(dot)
			}
			// mod: presence — tooltip with activity, status, and device info
			mods.SetupPresenceTooltip(ch.ctx, ch.name, u.ID, 0)
		}
	} else {
		ch.avatar.SetIconName("avatar-default-symbolic")
		ch.avatar.SetFromURL(gtkcord.InjectAvatarSize(channel.IconURL()))
	}

	ch.updateReadIndicator(channel)
}

func (ch *Channel) updateReadIndicator(channel *discord.Channel) {
	state := gtkcord.FromContext(ch.ctx)
	unread := state.ChannelCountUnreads(channel.ID, ningen.UnreadOpts{})

	if unread == 0 {
		ch.readIndicator.SetText("")
	} else {
		ch.readIndicator.SetText(fmt.Sprintf("(%d)", unread))
	}

	ch.InvalidateSort()
}

// LastMessageID queries the local state for the channel's last message ID.
func (ch *Channel) LastMessageID() discord.MessageID {
	state := gtkcord.FromContext(ch.ctx)
	return state.LastMessage(ch.id)
}

// Name returns the current displaying name of the channel.
func (ch *Channel) Name() string {
	return ch.name.Text()
}

// InvalidateSort invalidates the sorting position of this channel within the
// major channel list.
func (ch *Channel) InvalidateSort() {
	ch.Changed()
}
