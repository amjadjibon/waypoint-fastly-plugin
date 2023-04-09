package main

import (
	sdk "github.com/hashicorp/waypoint-plugin-sdk"

	"github.com/amjadjibon/waypoint-fastly-plugin/builder"
	"github.com/amjadjibon/waypoint-fastly-plugin/platform"
	"github.com/amjadjibon/waypoint-fastly-plugin/registry"
	"github.com/amjadjibon/waypoint-fastly-plugin/release"
)

func main() {
	sdk.Main(sdk.WithComponents(
		&builder.Builder{},
		&registry.Registry{},
		&platform.Platform{},
		&release.Manager{},
	))
}
