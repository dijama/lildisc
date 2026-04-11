// Package icons generates and loads icon Gresource.
package icons

import (
	_ "embed"
	"log"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
)

//go:generate glib-compile-resources lildisc.gresource.xml
//go:generate ./windows/generate-icon.sh

//go:embed lildisc.gresource
var Resources []byte

func init() {
	resources, err := gio.NewResourceFromData(glib.NewBytesWithGo(Resources))
	if err != nil {
		log.Panicln("Failed to create resources: ", err)
	}
	gio.ResourcesRegister(resources)
}
