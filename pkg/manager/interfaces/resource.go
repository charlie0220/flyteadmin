package interfaces

import (
	"context"

	"github.com/lyft/flyteidl/gen/pb-go/flyteidl/admin"
)

// Interface for managing project, domain and workflow -specific attributes.
type ResourceInterface interface {
	ListAll(ctx context.Context, request admin.ListMatchableAttributesRequest) (
		*admin.ListMatchableAttributesResponse, error)
	GetResource(ctx context.Context, request ResourceRequest) (*ResourceResponse, error)

	UpdateProjectDomainAttributes(ctx context.Context, request admin.ProjectDomainAttributesUpdateRequest) (
		*admin.ProjectDomainAttributesUpdateResponse, error)
	GetProjectDomainAttributes(ctx context.Context, request admin.ProjectDomainAttributesGetRequest) (
		*admin.ProjectDomainAttributesGetResponse, error)
	DeleteProjectDomainAttributes(ctx context.Context, request admin.ProjectDomainAttributesDeleteRequest) (
		*admin.ProjectDomainAttributesDeleteResponse, error)

	UpdateWorkflowAttributes(ctx context.Context, request admin.WorkflowAttributesUpdateRequest) (
		*admin.WorkflowAttributesUpdateResponse, error)
	GetWorkflowAttributes(ctx context.Context, request admin.WorkflowAttributesGetRequest) (
		*admin.WorkflowAttributesGetResponse, error)
	DeleteWorkflowAttributes(ctx context.Context, request admin.WorkflowAttributesDeleteRequest) (
		*admin.WorkflowAttributesDeleteResponse, error)
}

// TODO we can move this to flyteidl, once we are exposing an endpoint
type ResourceRequest struct {
	Project      string
	Domain       string
	Workflow     string
	LaunchPlan   string
	ResourceType admin.MatchableResource
}

type ResourceResponse struct {
	Project      string
	Domain       string
	Workflow     string
	LaunchPlan   string
	ResourceType string
	Attributes   *admin.MatchingAttributes
}
