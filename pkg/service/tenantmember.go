package service

import (
	"context"
	"fmt"
	"log/slog"

	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
)

type tenantMemberService struct {
	tenantMemberStore api.Storage[*v1.TenantMember]
	tenantStore       api.Storage[*v1.Tenant]
	log               *slog.Logger
}

func NewTenantMemberService(l *slog.Logger, tds TenantDataStore, tmds TenantMemberDataStore) *tenantMemberService {
	return &tenantMemberService{
		tenantMemberStore: NewStorageStatusWrapper(tmds),
		tenantStore:       NewStorageStatusWrapper(tds),
		log:               l,
	}
}

func (s *tenantMemberService) Create(ctx context.Context, req *v1.TenantMemberServiceCreateRequest) (*v1.TenantMemberServiceCreateResponse, error) {
	tenantMember := req.TenantMember

	_, err := s.tenantStore.Get(ctx, tenantMember.GetTenantId())
	if err != nil && errorutil.IsNotFound(err) {
		return nil, errorutil.NotFound("unable to find tenant:%s for tenantMember", tenantMember.GetTenantId())
	}
	if err != nil {
		return nil, err
	}

	// allow create without sending Meta
	if tenantMember.Meta == nil {
		tenantMember.Meta = &v1.Meta{}
	}

	err = s.tenantMemberStore.Create(ctx, tenantMember)

	return &v1.TenantMemberServiceCreateResponse{TenantMember: tenantMember}, err
}

func (s *tenantMemberService) Update(ctx context.Context, req *v1.TenantMemberServiceUpdateRequest) (*v1.TenantMemberServiceUpdateResponse, error) {
	tenantMember := req.TenantMember

	old, err := s.tenantMemberStore.Get(ctx, tenantMember.Meta.Id)
	if err != nil {
		return nil, err
	}

	if old.TenantId != tenantMember.TenantId {
		return nil, errorutil.InvalidArgument("updating the tenant id of a tenant member is not allowed")
	}
	if old.MemberId != tenantMember.MemberId {
		return nil, errorutil.InvalidArgument("updating the member id of a tenant member is not allowed")
	}
	if old.Namespace != tenantMember.Namespace {
		return nil, errorutil.InvalidArgument("updating the namespace of a tenant member is not allowed")
	}

	err = s.tenantMemberStore.Update(ctx, tenantMember)

	return &v1.TenantMemberServiceUpdateResponse{TenantMember: tenantMember}, err
}

func (s *tenantMemberService) Delete(ctx context.Context, req *v1.TenantMemberServiceDeleteRequest) (*v1.TenantMemberServiceDeleteResponse, error) {
	tenantMember := &v1.TenantMember{
		Meta: &v1.Meta{Id: req.Id},
	}

	err := s.tenantMemberStore.Delete(ctx, tenantMember.Meta.Id)

	return &v1.TenantMemberServiceDeleteResponse{TenantMember: tenantMember}, err
}

func (s *tenantMemberService) Get(ctx context.Context, req *v1.TenantMemberServiceGetRequest) (*v1.TenantMemberServiceGetResponse, error) {
	tenantMember, err := s.tenantMemberStore.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &v1.TenantMemberServiceGetResponse{TenantMember: tenantMember}, err
}

func (s *tenantMemberService) List(ctx context.Context, req *v1.TenantMemberServiceListRequest) (*v1.TenantMemberServiceListResponse, error) {
	filter := map[string]any{
		"COALESCE(tenantmember ->> 'namespace', '')": req.Namespace,
	}

	if req.TenantId != nil {
		filter["tenantmember ->> 'tenant_id'"] = req.TenantId
	}
	if req.MemberId != nil {
		filter["tenantmember ->> 'member_id'"] = req.MemberId
	}
	for key, value := range req.Annotations {
		// select * from tenantMember where tenantMember -> 'meta' -> 'annotations' ->>  'metal-stack.io/role' = 'owner';
		f := fmt.Sprintf("tenantmember -> 'meta' -> 'annotations' ->> '%s'", key)
		filter[f] = value
	}

	res, _, err := s.tenantMemberStore.Find(ctx, nil, filter)
	if err != nil {
		return nil, err
	}

	resp := new(v1.TenantMemberServiceListResponse)
	resp.TenantMembers = append(resp.TenantMembers, res...)

	return resp, nil
}
