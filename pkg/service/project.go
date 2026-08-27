package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
)

type projectService struct {
	projectStore       api.Storage[*v1.Project]
	projectMemberStore api.Storage[*v1.ProjectMember]
	tenantStore        api.Storage[*v1.Tenant]
	log                *slog.Logger
}

func NewProjectService(l *slog.Logger, pds ProjectDataStore, pmds ProjectMemberDataStore, tds TenantDataStore) *projectService {
	return &projectService{
		projectStore:       NewStorageStatusWrapper(pds),
		projectMemberStore: NewStorageStatusWrapper(pmds),
		tenantStore:        NewStorageStatusWrapper(tds),
		log:                l,
	}
}

func (s *projectService) Create(ctx context.Context, req *v1.ProjectServiceCreateRequest) (*v1.ProjectServiceCreateResponse, error) {
	project := req.Project

	_, err := s.tenantStore.Get(ctx, project.GetTenantId())
	if err != nil && errorutil.IsNotFound(err) {
		return nil, errorutil.NotFound("unable to find tenant:%s for project", project.GetTenantId())
	}
	if err != nil {
		return nil, err
	}

	// allow create without sending Meta
	if project.Meta == nil {
		project.Meta = &v1.Meta{}
	}
	err = s.projectStore.Create(ctx, project)
	return &v1.ProjectServiceCreateResponse{Project: project}, err
}
func (s *projectService) Update(ctx context.Context, req *v1.ProjectServiceUpdateRequest) (*v1.ProjectServiceUpdateResponse, error) {
	old, err := s.projectStore.Get(ctx, req.Project.Meta.Id)
	if err != nil {
		return nil, err
	}
	project := req.Project
	if old.TenantId != project.TenantId {
		return nil, errorutil.InvalidArgument("update tenant of project:%s is not allowed", project.Meta.Id)
	}
	err = s.projectStore.Update(ctx, project)
	return &v1.ProjectServiceUpdateResponse{Project: project}, err
}
func (s *projectService) Delete(ctx context.Context, req *v1.ProjectServiceDeleteRequest) (*v1.ProjectServiceDeleteResponse, error) {
	project := &v1.Project{
		Meta: &v1.Meta{Id: req.Id},
	}
	filter := map[string]any{
		"projectmember ->> 'project_id'": project.Meta.Id,
	}
	memberships, _, err := s.projectMemberStore.Find(ctx, nil, filter)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, m := range memberships {
		ids = append(ids, m.Meta.Id)
	}

	if len(ids) > 0 {
		err = s.projectMemberStore.DeleteAll(ctx, ids...)
		if err != nil {
			return nil, err
		}
	}
	err = s.projectStore.Delete(ctx, project.Meta.Id)
	if err != nil {
		return nil, err
	}
	return &v1.ProjectServiceDeleteResponse{Project: project}, err
}
func (s *projectService) Get(ctx context.Context, req *v1.ProjectServiceGetRequest) (*v1.ProjectServiceGetResponse, error) {
	project, err := s.projectStore.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.ProjectServiceGetResponse{Project: project}, nil
}
func (s *projectService) GetHistory(ctx context.Context, req *v1.ProjectServiceGetHistoryRequest) (*v1.ProjectServiceGetHistoryResponse, error) {
	project := &v1.Project{}
	at := req.At.AsTime()
	err := s.projectStore.GetHistory(ctx, req.Id, at, project)
	if err != nil {
		return nil, err
	}
	return &v1.ProjectServiceGetHistoryResponse{Project: project}, err
}
func (s *projectService) List(ctx context.Context, req *v1.ProjectServiceListRequest) (*v1.ProjectServiceListResponse, error) {
	var filters []any

	mapFilter := make(map[string]any)
	if req.Id != nil {
		mapFilter["id"] = req.Id
	}
	if req.Name != nil {
		mapFilter["project ->> 'name'"] = req.Name
	}
	if req.Description != nil {
		mapFilter["project ->> 'description'"] = req.Description
	}
	if req.TenantId != nil {
		mapFilter["project ->> 'tenant_id'"] = req.TenantId
	}
	for key, value := range req.Annotations {
		// select * from project where project -> 'meta' -> 'annotations' ->>  'metal-stack.io/admitted' = 'true';
		f := fmt.Sprintf("project -> 'meta' -> 'annotations' ->> '%s'", key)
		mapFilter[f] = value
	}

	if len(mapFilter) > 0 {
		filters = append(filters, mapFilter)
	}

	if len(req.Labels) > 0 {
		var contains []string

		for _, label := range req.Labels {
			contains = append(contains, strconv.Quote(label))
		}

		// select * from projects where project -> 'meta' -> 'labels' @> '["a=b","c=d"]';
		labelFilter := fmt.Sprintf(`project -> 'meta' -> 'labels' @> '[%s]'`, strings.Join(contains, ","))

		filters = append(filters, labelFilter)
	}

	res, nextPage, err := s.projectStore.Find(ctx, req.Paging, filters...)
	if err != nil {
		return nil, err
	}
	resp := new(v1.ProjectServiceListResponse)
	resp.Projects = append(resp.Projects, res...)
	resp.NextPage = nextPage
	return resp, nil
}
