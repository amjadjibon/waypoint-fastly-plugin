package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/framework/resource"
	sdk "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/amjadjibon/waypoint-fastly-plugin/registry"
)

type DeployConfig struct {
	Region string "hcl:directory,optional"
}

type Platform struct {
	config DeployConfig
}

func (p *Platform) Config() (interface{}, error) {
	return &p.config, nil
}

func (p *Platform) ConfigSet(config interface{}) error {
	c, ok := config.(*DeployConfig)
	if !ok {
		// The Waypoint SDK should ensure this never gets hit
		return fmt.Errorf("Expected *DeployConfig as parameter")
	}

	// validate the config
	if c.Region == "" {
		return fmt.Errorf("Region must be set to a valid directory")
	}

	return nil
}

func (p *Platform) getConnectContext() (interface{}, error) {
	return nil, nil
}

func (p *Platform) resourceManager(log hclog.Logger, dcr *component.DeclaredResourcesResp) *resource.Manager {
	return resource.NewManager(
		resource.WithLogger(log.Named("resource_manager")),
		resource.WithValueProvider(p.getConnectContext),
		resource.WithDeclaredResourcesResp(dcr),
		resource.WithResource(resource.NewResource(
			resource.WithName("template_example"),
			resource.WithState(&Resource_Deployment{}),
			resource.WithCreate(p.resourceDeploymentCreate),
			resource.WithDestroy(p.resourceDeploymentDestroy),
			resource.WithStatus(p.resourceDeploymentStatus),
			resource.WithPlatform("template_platform"),                                         // Update this to match your plugins platform, like Kubernetes
			resource.WithCategoryDisplayHint(sdk.ResourceCategoryDisplayHint_INSTANCE_MANAGER), // This is meant for the UI to determine what kind of icon to show
		)),
	)
}

func (p *Platform) DeployFunc() interface{} {
	return p.deploy
}

func (p *Platform) StatusFunc() interface{} {
	return p.status
}

func (p *Platform) deploy(
	ctx context.Context,
	ui terminal.UI,
	log hclog.Logger,
	dcr *component.DeclaredResourcesResp,
	artifact *registry.Artifact,
) (*Deployment, error) {
	u := ui.Status()
	defer func() {
		_ = u.Close()
	}()

	u.Update("Deploy application")

	var result Deployment

	// Create our resource manager and create deployment resources
	rm := p.resourceManager(log, dcr)

	if err := rm.CreateAll(
		ctx, log, u, ui,
		artifact, &result,
	); err != nil {
		return nil, err
	}

	// Store our resource state
	result.ResourceState = rm.State()

	u.Update("Application deployed")

	return &Deployment{}, nil
}

func (p *Platform) status(
	ctx context.Context,
	ji *component.JobInfo,
	ui terminal.UI,
	log hclog.Logger,
	deployment *Deployment,
) (*sdk.StatusReport, error) {
	sg := ui.StepGroup()
	s := sg.Add("Checking the status of the deployment...")

	rm := p.resourceManager(log, nil)

	if deployment.ResourceState == nil {
		err := rm.Resource("deployment").SetState(&Resource_Deployment{
			Name: deployment.Id,
		})
		if err != nil {
			return nil, err
		}
	} else {
		// Load our set state
		if err := rm.LoadState(deployment.ResourceState); err != nil {
			return nil, err
		}
	}

	// This will call the StatusReport func on every defined resource in ResourceManager
	report, err := rm.StatusReport(ctx, log, sg, ui)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resource manager failed to generate resource statuses: %s", err)
	}

	report.Health = sdk.StatusReport_UNKNOWN
	s.Update("Deployment is currently not implemented!")
	s.Done()

	return report, nil
}

func (p *Platform) resourceDeploymentCreate(
	ctx context.Context,
	log hclog.Logger,
	st terminal.Status,
	ui terminal.UI,
	artifact *registry.Artifact,
	result *Deployment,
) error {
	// Create your deployment resource here!
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	appPath := filepath.Join(cwd, "app.js")
	appFile, err := os.Open(appPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = appFile.Close()
	}()

	// Create a new Fastly API client
	client, err := fastly.NewClient(os.Getenv("FASTLY_API_TOKEN"))
	if err != nil {
		return err
	}

	// Define the Fastly Compute@edge service configuration
	serviceInput := &fastly.CreateServiceInput{
		Name:    os.Getenv("FASTLY_SERVICE_NAME"),
		Type:    "compute",
		Comment: "Created by Waypoint",
	}

	// Create the Fastly Compute@edge service
	service, err := client.CreateService(serviceInput)
	if err != nil {
		return err
	}

	// Define the Fastly Compute@edge backend configuration
	backendInput := &fastly.CreateBackendInput{
		Name:    os.Getenv("BACKEND_NAME"),
		Address: os.Getenv("BACKEND_URL"),
		Port:    443,
		Version: int(service.ActiveVersion),
	}

	// Create the Fastly Compute@edge backend
	_, err = client.CreateBackend(backendInput)
	if err != nil {
		return err
	}

	// Define the Fastly Compute@edge domain configuration
	domainInput := &fastly.CreateDomainInput{
		Name:    os.Getenv("APP_DOMAIN"),
		Version: int(service.ActiveVersion),
	}

	// Create the Fastly Compute@edge domain
	_, err = client.CreateDomain(domainInput)
	if err != nil {
		return err
	}

	result.Id = service.ID
	result.Name = service.Name

	ui.Output(fmt.Sprintf("Deployed Node.js application to Fastly Compute@edge service %s", service.ID), terminal.WithSuccessStyle())

	return nil
}

func (p *Platform) resourceDeploymentStatus(
	ctx context.Context,
	ui terminal.UI,
	sg terminal.StepGroup,
	artifact *registry.Artifact,
) error {
	// deployment status code here
	client, _ := fastly.NewClient(os.Getenv("FASTLY_API_TOKEN"))

	service, err := client.GetServiceDetails(&fastly.GetServiceInput{
		ID: os.Getenv("FASTLY_SERVICE_ID"),
	})

	if err != nil {
		return err
	}

	if service.Version.ServiceID == os.Getenv("FASTLY_SERVICE_ID") {
		s := sg.Add("Fastly Compute@edge service is active")
		s.Update("Fastly Compute@edge service is active")
		s.Done()
	} else {
		s := sg.Add("Fastly Compute@edge service is inactive")
		s.Update("Fastly Compute@edge service is inactive")
		s.Done()
	}

	return nil
}
