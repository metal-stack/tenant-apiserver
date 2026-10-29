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

type projectConfigService struct {
	projectConfigStore api.Storage[*v1.ProjectConfig]
	projectStore       api.Storage[*v1.Project]
	log                *slog.Logger
}

func NewProjectConfigService(l *slog.Logger, pcds ProjectConfigDataStore, pds ProjectDataStore) *projectConfigService {
	return &projectConfigService{
		projectConfigStore: NewStorageStatusWrapper(pcds),
		projectStore:       NewStorageStatusWrapper(pds),
		log:                l,
	}
}

func (s *projectConfigService) Create(ctx context.Context, req *v1.ProjectConfigServiceCreateRequest) (*v1.ProjectConfigServiceCreateResponse, error) {
	projectConfig := req.ProjectConfig

	_, err := s.projectStore.Get(ctx, projectConfig.GetProjectId())
	if err != nil && errorutil.IsNotFound(err) {
		return nil, errorutil.NotFound("unable to find project:%s for projectConfig", projectConfig.GetProjectId())
	}
	if err != nil {
		return nil, err
	}

	// allow create without sending Meta
	if projectConfig.Meta == nil {
		projectConfig.Meta = &v1.Meta{}
	}

	err = s.projectConfigStore.Create(ctx, projectConfig)
	if err != nil {
		return nil, err
	}

	return &v1.ProjectConfigServiceCreateResponse{ProjectConfig: projectConfig}, nil
}

func (s *projectConfigService) Update(ctx context.Context, req *v1.ProjectConfigServiceUpdateRequest) (*v1.ProjectConfigServiceUpdateResponse, error) {
	old, err := s.projectConfigStore.Get(ctx, req.ProjectConfig.GetMeta().GetId())
	if err != nil {
		return nil, err
	}

	projectConfig := req.ProjectConfig

	if old.ProjectId != projectConfig.ProjectId {
		return nil, errorutil.InvalidArgument("update project of projectConfig:%s is not allowed", projectConfig.GetMeta().GetId())
	}

	err = s.projectConfigStore.Update(ctx, projectConfig)
	if err != nil {
		return nil, err
	}

	return &v1.ProjectConfigServiceUpdateResponse{ProjectConfig: projectConfig}, nil
}

func (s *projectConfigService) Delete(ctx context.Context, req *v1.ProjectConfigServiceDeleteRequest) (*v1.ProjectConfigServiceDeleteResponse, error) {
	projectConfig := &v1.ProjectConfig{
		Meta: &v1.Meta{Id: req.Id},
	}

	err := s.projectConfigStore.Delete(ctx, projectConfig.Meta.Id)
	if err != nil {
		return nil, err
	}

	return &v1.ProjectConfigServiceDeleteResponse{ProjectConfig: projectConfig}, nil
}

func (s *projectConfigService) Get(ctx context.Context, req *v1.ProjectConfigServiceGetRequest) (*v1.ProjectConfigServiceGetResponse, error) {
	projectConfig, err := s.projectConfigStore.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &v1.ProjectConfigServiceGetResponse{ProjectConfig: projectConfig}, nil
}

func (s *projectConfigService) List(ctx context.Context, req *v1.ProjectConfigServiceListRequest) (*v1.ProjectConfigServiceListResponse, error) {
	var filters []any

	mapFilter := make(map[string]any)
	if req.Id != nil {
		mapFilter["id"] = req.Id
	}
	if req.Name != nil {
		mapFilter["projectconfig ->> 'name'"] = req.Name
	}
	if req.ProjectId != nil {
		mapFilter["projectconfig ->> 'project_id'"] = req.ProjectId
	}
	for key, value := range req.Annotations {
		// select * from projectconfigs where projectconfig -> 'meta' -> 'annotations' ->>  'metal-stack.io/admitted' = 'true';
		f := fmt.Sprintf("projectconfig -> 'meta' -> 'annotations' ->> '%s'", key)
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

		// select * from projectconfigs where projectconfig -> 'meta' -> 'labels' @> '["a=b","c=d"]';
		labelFilter := fmt.Sprintf(`projectconfig -> 'meta' -> 'labels' @> '[%s]'`, strings.Join(contains, ","))

		filters = append(filters, labelFilter)
	}

	res, nextPage, err := s.projectConfigStore.Find(ctx, req.Paging, filters...)
	if err != nil {
		return nil, err
	}

	resp := new(v1.ProjectConfigServiceListResponse)
	resp.ProjectConfig = append(resp.ProjectConfig, res...)
	resp.NextPage = nextPage

	return resp, nil
}
