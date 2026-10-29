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

type tenantConfigService struct {
	tenantConfigStore api.Storage[*v1.TenantConfig]
	tenantStore       api.Storage[*v1.Tenant]
	log               *slog.Logger
}

func NewTenantConfigService(l *slog.Logger, tcds TenantConfigDataStore, tds TenantDataStore) *tenantConfigService {
	return &tenantConfigService{
		tenantConfigStore: NewStorageStatusWrapper(tcds),
		tenantStore:       NewStorageStatusWrapper(tds),
		log:               l,
	}
}

func (s *tenantConfigService) Create(ctx context.Context, req *v1.TenantConfigServiceCreateRequest) (*v1.TenantConfigServiceCreateResponse, error) {
	tenantConfig := req.TenantConfig

	_, err := s.tenantStore.Get(ctx, tenantConfig.GetTenantId())
	if err != nil && errorutil.IsNotFound(err) {
		return nil, errorutil.NotFound("unable to find tenant:%s for tenantConfig", tenantConfig.GetTenantId())
	}
	if err != nil {
		return nil, err
	}

	// allow create without sending Meta
	if tenantConfig.Meta == nil {
		tenantConfig.Meta = &v1.Meta{}
	}

	err = s.tenantConfigStore.Create(ctx, tenantConfig)
	if err != nil {
		return nil, err
	}

	return &v1.TenantConfigServiceCreateResponse{TenantConfig: tenantConfig}, nil
}

func (s *tenantConfigService) Update(ctx context.Context, req *v1.TenantConfigServiceUpdateRequest) (*v1.TenantConfigServiceUpdateResponse, error) {
	old, err := s.tenantConfigStore.Get(ctx, req.TenantConfig.GetMeta().GetId())
	if err != nil {
		return nil, err
	}

	tenantConfig := req.TenantConfig

	if old.TenantId != tenantConfig.TenantId {
		return nil, errorutil.InvalidArgument("update tenant of tenantConfig:%s is not allowed", tenantConfig.GetMeta().GetId())
	}

	err = s.tenantConfigStore.Update(ctx, tenantConfig)
	if err != nil {
		return nil, err
	}

	return &v1.TenantConfigServiceUpdateResponse{TenantConfig: tenantConfig}, nil
}

func (s *tenantConfigService) Delete(ctx context.Context, req *v1.TenantConfigServiceDeleteRequest) (*v1.TenantConfigServiceDeleteResponse, error) {
	tenantConfig := &v1.TenantConfig{
		Meta: &v1.Meta{Id: req.Id},
	}

	err := s.tenantConfigStore.Delete(ctx, tenantConfig.Meta.Id)
	if err != nil {
		return nil, err
	}

	return &v1.TenantConfigServiceDeleteResponse{TenantConfig: tenantConfig}, nil
}

func (s *tenantConfigService) Get(ctx context.Context, req *v1.TenantConfigServiceGetRequest) (*v1.TenantConfigServiceGetResponse, error) {
	tenantConfig, err := s.tenantConfigStore.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &v1.TenantConfigServiceGetResponse{TenantConfig: tenantConfig}, nil
}

func (s *tenantConfigService) List(ctx context.Context, req *v1.TenantConfigServiceListRequest) (*v1.TenantConfigServiceListResponse, error) {
	var filters []any

	mapFilter := make(map[string]any)
	if req.Id != nil {
		mapFilter["id"] = req.Id
	}
	if req.Name != nil {
		mapFilter["tenantconfig ->> 'name'"] = req.Name
	}
	if req.TenantId != nil {
		mapFilter["tenantconfig ->> 'tenant_id'"] = req.TenantId
	}
	for key, value := range req.Annotations {
		// select * from tenantconfigs where tenantconfig -> 'meta' -> 'annotations' ->>  'metal-stack.io/admitted' = 'true';
		f := fmt.Sprintf("tenantconfig -> 'meta' -> 'annotations' ->> '%s'", key)
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

		// select * from tenantconfigs where tenantconfig -> 'meta' -> 'labels' @> '["a=b","c=d"]';
		labelFilter := fmt.Sprintf(`tenantconfig -> 'meta' -> 'labels' @> '[%s]'`, strings.Join(contains, ","))

		filters = append(filters, labelFilter)
	}

	res, nextPage, err := s.tenantConfigStore.Find(ctx, req.Paging, filters...)
	if err != nil {
		return nil, err
	}

	resp := new(v1.TenantConfigServiceListResponse)
	resp.TenantConfig = append(resp.TenantConfig, res...)
	resp.NextPage = nextPage

	return resp, nil
}
