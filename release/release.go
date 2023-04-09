package release

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/framework/resource"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/amjadjibon/waypoint-fastly-plugin/registry"
)

type Config struct {
	Active bool "hcl:directory,optional"
}

type Manager struct {
	config Config
}

func (rm *Manager) Config() (interface{}, error) {
	return &rm.config, nil
}

func (rm *Manager) ConfigSet(config interface{}) error {
	_, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("expected *ReleaseConfig as parameter")
	}
	return nil
}

func (rm *Manager) ReleaseFunc() interface{} {
	return rm.release
}

func (rm *Manager) StatusFunc() interface{} {
	return rm.status
}

func (rm *Manager) getConnectContext() (interface{}, error) {
	return nil, nil
}

func (rm *Manager) resourceManager(log hclog.Logger, dcr *component.DeclaredResourcesResp) *resource.Manager {
	return resource.NewManager(
		resource.WithLogger(log.Named("resource_manager")),
		resource.WithValueProvider(rm.getConnectContext),
		resource.WithDeclaredResourcesResp(dcr),
		resource.WithResource(resource.NewResource(
			resource.WithName("template_example"),
			resource.WithState(&Resource_Release{}),
			resource.WithCreate(rm.resourceReleaseCreate),
			resource.WithDestroy(rm.resourceReleaseDestroy),
			resource.WithStatus(rm.resourceReleaseStatus),
			resource.WithPlatform("template_platform"),                                         // Update this to match your plugins platform, like Kubernetes
			resource.WithCategoryDisplayHint(sdk.ResourceCategoryDisplayHint_INSTANCE_MANAGER), // This is meant for the UI to determine what kind of icon to show
		)),
	)
}

func (rm *Manager) release(
	ctx context.Context,
	log hclog.Logger,
	dcr *component.DeclaredResourcesResp,
	ui terminal.UI,
	artifact *registry.Artifact,
) (*Release, error) {
	u := ui.Status()
	defer u.Close()
	u.Update("Release application")

	var result *Release

	// Create our resource manager and create deployment resources
	r := rm.resourceManager(log, dcr)

	// These params must match exactly to your resource manager functions. Otherwise
	// they will not be invoked during CreateAll()
	if err := r.CreateAll(
		ctx, log, u, ui,
		artifact, &result,
	); err != nil {
		return nil, err
	}

	// Store our resource state
	result.ResourceState = r.State()

	u.Update("Application deployed")

	return result, nil
}

func (rm *Manager) status(
	ctx context.Context,
	ji *component.JobInfo,
	log hclog.Logger,
	ui terminal.UI,
	artifact *registry.Artifact,
	release *Release,
) (*sdk.StatusReport, error) {
	sg := ui.StepGroup()
	s := sg.Add("Checking the status of the release...")

	r := rm.resourceManager(log, nil)

	// If we don't have resource state, this state is from an older version
	// and we need to manually recreate it.
	if release.ResourceState == nil {
		r.Resource("release").SetState(&Resource_Release{
			Name: release.Id,
		})
	} else {
		// Load our set state
		if err := r.LoadState(release.ResourceState); err != nil {
			return nil, err
		}
	}

	// This will call the StatusReport func on every defined resource in ResourceManager
	report, err := r.StatusReport(ctx, log, sg, ui)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resource manager failed to generate resource statuses: %s", err)
	}

	report.Health = sdk.StatusReport_UNKNOWN
	s.Update("Release is currently not implemented")
	s.Done()

	return report, nil
}

func (rm *Manager) resourceReleaseCreate(
	ctx context.Context,
	log hclog.Logger,
	st terminal.Status,
	ui terminal.UI,
	artifact *registry.Artifact,
	result *Release,
) error {

	return nil
}

func (rm *Manager) resourceReleaseStatus(
	ctx context.Context,
	ui terminal.UI,
	sg terminal.StepGroup,
	artifact *registry.Artifact,
) error {
	return nil
}
