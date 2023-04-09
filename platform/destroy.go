package platform

import (
	"context"
	"fmt"
	"os"

	"github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

func (p *Platform) DestroyFunc() interface{} {
	return p.destroy
}

func (p *Platform) destroy(
	ctx context.Context,
	ui terminal.UI,
	log hclog.Logger,
	deployment *Deployment,
) error {
	sg := ui.StepGroup()
	defer sg.Wait()

	rm := p.resourceManager(log, nil)
	if deployment.ResourceState == nil {
		err := rm.Resource("deployment").SetState(&Resource_Deployment{
			Name: deployment.Name,
		})
		if err != nil {
			return err
		}
	} else {
		if err := rm.LoadState(deployment.ResourceState); err != nil {
			return err
		}
	}

	// Destroy
	return rm.DestroyAll(ctx, log, sg, ui)
}

func (p *Platform) resourceDeploymentDestroy(
	ctx context.Context,
	log hclog.Logger,
	sg terminal.StepGroup,
	ui terminal.UI,
) error {
	// Get the deployment name
	deploymentName := p.resourceManager(log, nil).Resource("deployment").State().(*Resource_Deployment).Name

	// Create a new Fastly API client
	client, err := fastly.NewClient(os.Getenv("FASTLY_API_TOKEN"))
	if err != nil {
		return fmt.Errorf("failed to create Fastly client: %w", err)
	}

	// Find the service ID for the deployment
	serviceID, err := p.getServiceID(ctx, client, deploymentName)
	if err != nil {
		return fmt.Errorf("failed to find Fastly service ID: %w", err)
	}

	// Delete the service
	if err := client.DeleteService(&fastly.DeleteServiceInput{ID: serviceID}); err != nil {
		return fmt.Errorf("failed to delete Fastly service: %w", err)
	}

	ui.Status().Update("Fastly service deleted")

	return nil
}

func (p *Platform) getServiceID(ctx context.Context, client *fastly.Client, deploymentName string) (string, error) {
	services, err := client.ListServices(&fastly.ListServicesInput{})
	if err != nil {
		return "", fmt.Errorf("failed to list Fastly services: %w", err)
	}

	for _, s := range services {
		if s.Name == deploymentName {
			return s.ID, nil
		}
	}

	return "", fmt.Errorf("service not found: %s", deploymentName)
}
