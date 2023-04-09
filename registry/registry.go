package registry

import (
	"context"
	"fmt"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"

	"github.com/amjadjibon/waypoint-fastly-plugin/builder"
)

type Config struct {
	Name    string "hcl:name"
	Version string "hcl:version"
}

type Registry struct {
	config Config
}

func (r *Registry) Config() (interface{}, error) {
	return &r.config, nil
}

func (r *Registry) ConfigSet(config interface{}) error {
	c, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("expected *RegisterConfig as parameter")
	}

	// validate the config
	if c.Name == "" {
		return fmt.Errorf("name must be set to a valid directory")
	}

	return nil
}

func (r *Registry) PushFunc() interface{} {
	return r.push
}

func (r *Registry) push(ctx context.Context, ui terminal.UI, binary *builder.Binary) (*Artifact, error) {
	u := ui.Status()
	defer func() {
		_ = u.Close()
	}()

	u.Update("Pushing binary to registry")

	return &Artifact{}, nil
}

var _ component.Registry = (*Registry)(nil)
