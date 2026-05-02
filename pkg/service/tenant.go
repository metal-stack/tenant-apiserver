package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
)

type tenantService struct {
	log               *slog.Logger
	tenantStore       api.Storage[*v1.Tenant]
	tenantMemberStore api.Storage[*v1.TenantMember]
	db                *sqlx.DB
}

var (
	projectMembers = api.Entity(&v1.ProjectMember{})
	tenantMembers  = api.Entity(&v1.TenantMember{})
	projects       = api.Entity(&v1.Project{})
	tenants        = api.Entity(&v1.Tenant{})
)

func NewTenantService(db *sqlx.DB, l *slog.Logger, tds TenantDataStore, tmds TenantMemberDataStore) *tenantService {
	return &tenantService{
		db:                db,
		tenantStore:       NewStorageStatusWrapper(tds),
		tenantMemberStore: NewStorageStatusWrapper(tmds),
		log:               l,
	}
}

func (s *tenantService) Create(ctx context.Context, req *v1.TenantServiceCreateRequest) (*v1.TenantServiceCreateResponse, error) {
	tenant := req.Tenant
	// allow create without sending Meta
	if tenant.Meta == nil {
		tenant.Meta = &v1.Meta{}
	}
	err := s.tenantStore.Create(ctx, tenant)
	return &v1.TenantServiceCreateResponse{Tenant: tenant}, err
}
func (s *tenantService) Update(ctx context.Context, req *v1.TenantServiceUpdateRequest) (*v1.TenantServiceUpdateResponse, error) {
	tenant := req.Tenant
	err := s.tenantStore.Update(ctx, tenant)
	return &v1.TenantServiceUpdateResponse{Tenant: tenant}, err
}

func (s *tenantService) Delete(ctx context.Context, req *v1.TenantServiceDeleteRequest) (*v1.TenantServiceDeleteResponse, error) {
	tenant := &v1.Tenant{
		Meta: &v1.Meta{Id: req.Id},
	}
	tenantIsHostFilter := map[string]any{
		"tenantmember ->> 'tenant_id'": tenant.Meta.Id,
	}
	tenantIsMemberFilter := map[string]any{
		"tenantmember ->> 'member_id'": tenant.Meta.Id,
	}
	tenantIsHostMemberships, _, err := s.tenantMemberStore.Find(ctx, nil, tenantIsHostFilter)
	if err != nil {
		return nil, err
	}
	tenantIsMemberMemberships, _, err := s.tenantMemberStore.Find(ctx, nil, tenantIsMemberFilter)
	if err != nil {
		return nil, err
	}

	unionMap := make(map[string]bool)
	for _, m := range tenantIsHostMemberships {
		unionMap[m.Meta.Id] = true
	}
	for _, m := range tenantIsMemberMemberships {
		unionMap[m.Meta.Id] = true
	}

	var ids []string
	for k := range unionMap {
		ids = append(ids, k)
	}

	if len(ids) > 0 {
		err = s.tenantMemberStore.DeleteAll(ctx, ids...)
		if err != nil {
			return nil, err
		}
	}
	err = s.tenantStore.Delete(ctx, tenant.Meta.Id)
	if err != nil {
		return nil, err
	}
	return &v1.TenantServiceDeleteResponse{Tenant: tenant}, nil
}

func (s *tenantService) Get(ctx context.Context, req *v1.TenantServiceGetRequest) (*v1.TenantServiceGetResponse, error) {
	tenant, err := s.tenantStore.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	// response with entity, no error
	return &v1.TenantServiceGetResponse{Tenant: tenant}, err
}

func (s *tenantService) GetHistory(ctx context.Context, req *v1.TenantServiceGetHistoryRequest) (*v1.TenantServiceGetHistoryResponse, error) {
	tenant := &v1.Tenant{}
	at := req.At.AsTime()
	s.log.Info("getHistory", "id", req.Id, "at", at)
	err := s.tenantStore.GetHistory(ctx, req.Id, at, tenant)
	if err != nil {
		return nil, err
	}

	// response with entity, no error
	return &v1.TenantServiceGetHistoryResponse{Tenant: tenant}, err
}

func (s *tenantService) List(ctx context.Context, req *v1.TenantServiceListRequest) (*v1.TenantServiceListResponse, error) {
	var filters []any

	mapFilter := make(map[string]any)
	if req.Id != nil {
		mapFilter["id"] = req.GetId()
	}
	if req.Name != nil {
		mapFilter["tenant ->> 'name'"] = req.GetName()
	}
	for key, value := range req.Annotations {
		// select * from tenants where tenant -> 'meta' -> 'annotations' ->>  'metal-stack.io/admitted' = 'true';
		f := fmt.Sprintf("tenant -> 'meta' -> 'annotations' ->> '%s'", key)
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

		// select * from tenants where tenant -> 'meta' -> 'labels' @> '["a=b","c=d"]';
		labelFilter := fmt.Sprintf(`tenant -> 'meta' -> 'labels' @> '[%s]'`, strings.Join(contains, ","))

		filters = append(filters, labelFilter)
	}
	res, nextPage, err := s.tenantStore.Find(ctx, req.Paging, filters...)
	if err != nil {
		return nil, err
	}
	resp := new(v1.TenantServiceListResponse)
	resp.Tenants = append(resp.Tenants, res...)
	resp.NextPage = nextPage
	return resp, nil
}

var (
	queryDirectProjectParticipations = sq.Select(
		projects.JSONField(),
		projectMembers.JSONField()+"->'meta'->>'annotations' AS project_membership_annotations",
	).
		From(projectMembers.TableName()).
		Join(projects.TableName() + " ON " + projects.TableName() + ".id = " + projectMembers.JSONField() + "->>'project_id'").
		Where(projectMembers.JSONField() + "->>'tenant_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + projectMembers.JSONField() + "->> 'namespace', '') = :namespace")

	queryInheritedProjectParticipations = sq.Select(
		projects.JSONField(),
		tenantMembers.JSONField()+"->'meta'->>'annotations' AS tenant_membership_annotations",
	).
		From(tenantMembers.TableName()).
		Join(projects.TableName() + " ON " + projects.JSONField() + "->>'tenant_id' = " + tenantMembers.JSONField() + "->>'tenant_id'").
		Where(tenantMembers.JSONField() + "->>'member_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + tenantMembers.JSONField() + "->> 'namespace', '') = :namespace")
)

// FindParticipatingProjects returns all projects in which a member participates.
// This includes projects in which the member is explicitly participating through a project membership but may also
// include memberships, which are inherited by the tenant membership.
func (s *tenantService) FindParticipatingProjects(ctx context.Context, req *v1.TenantServiceFindParticipatingProjectsRequest) (*v1.TenantServiceFindParticipatingProjectsResponse, error) {
	type result struct {
		Project                      *v1.Project
		TenantMembershipAnnotations  []byte `db:"tenant_membership_annotations"`
		ProjectMembershipAnnotations []byte `db:"project_membership_annotations"`
	}

	var (
		res       []*v1.ProjectWithMembershipAnnotations
		resultMap = map[string]*v1.ProjectWithMembershipAnnotations{}

		input = map[string]any{"tenantId": req.TenantId, "namespace": req.Namespace}

		resultFn = func(e result) error {
			p, ok := resultMap[e.Project.Meta.Id]
			if !ok {
				p = &v1.ProjectWithMembershipAnnotations{
					Project: e.Project,
				}
			}

			if e.TenantMembershipAnnotations != nil {
				err := json.Unmarshal(e.TenantMembershipAnnotations, &p.TenantAnnotations)
				if err != nil {
					return err
				}
			}

			if e.ProjectMembershipAnnotations != nil {
				err := json.Unmarshal(e.ProjectMembershipAnnotations, &p.ProjectAnnotations)
				if err != nil {
					return err
				}
			}

			resultMap[e.Project.Meta.Id] = p

			return nil
		}
	)

	err := postgres.RunQuery(ctx, s.log, s.db, queryDirectProjectParticipations, input, resultFn)
	if err != nil {
		return nil, err
	}

	includeInherited := true
	if req.IncludeInherited != nil {
		includeInherited = *req.IncludeInherited
	}

	if includeInherited {
		err := postgres.RunQuery(ctx, s.log, s.db, queryInheritedProjectParticipations, input, resultFn)
		if err != nil {
			return nil, err
		}
	}

	for _, p := range resultMap {
		res = append(res, p)
	}

	return &v1.TenantServiceFindParticipatingProjectsResponse{Projects: res}, nil
}

var (
	queryDirectTenantParticipations = sq.Select(
		tenants.JSONField(),
		tenantMembers.JSONField()+"->'meta'->>'annotations' AS tenant_membership_annotations",
	).
		From(tenantMembers.TableName()).
		Join(tenants.TableName() + " ON " + tenants.TableName() + ".id = " + tenantMembers.JSONField() + "->>'tenant_id'").
		Where(tenantMembers.JSONField() + "->>'member_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + tenantMembers.JSONField() + "->> 'namespace', '') = :namespace")

	queryInheritedTenantParticipations = sq.Select(
		tenants.JSONField(),
		projectMembers.JSONField()+"->'meta'->>'annotations' AS project_membership_annotations",
	).
		From(projectMembers.TableName()).
		Join(projects.TableName() + " ON " + projects.TableName() + ".id = " + projectMembers.JSONField() + "->>'project_id'").
		Join(tenants.TableName() + " ON " + tenants.TableName() + ".id = " + projects.JSONField() + "->>'tenant_id'").
		Where(projectMembers.JSONField() + "->>'tenant_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + projectMembers.JSONField() + "->> 'namespace', '') = :namespace")
)

// FindParticipatingTenants returns all tenants in which a member participates.
// This includes tenants in which the member is explicitly participating through a tenant membership but may also
// include memberships, which are inherited by the project memberships (e.g. through project invites).
func (s *tenantService) FindParticipatingTenants(ctx context.Context, req *v1.TenantServiceFindParticipatingTenantsRequest) (*v1.TenantServiceFindParticipatingTenantsResponse, error) {
	type result struct {
		Tenant                       *v1.Tenant
		TenantMembershipAnnotations  []byte `db:"tenant_membership_annotations"`
		ProjectMembershipAnnotations []byte `db:"project_membership_annotations"`
	}

	var (
		input = map[string]any{"tenantId": req.TenantId, "namespace": req.Namespace}

		res       []*v1.TenantWithMembershipAnnotations
		resultMap = map[string]*v1.TenantWithMembershipAnnotations{}

		resultFn = func(e result) error {
			t, ok := resultMap[e.Tenant.Meta.Id]
			if !ok {
				t = &v1.TenantWithMembershipAnnotations{
					Tenant: e.Tenant,
				}
			}

			if e.TenantMembershipAnnotations != nil {
				err := json.Unmarshal(e.TenantMembershipAnnotations, &t.TenantAnnotations)
				if err != nil {
					return err
				}
			}

			if e.ProjectMembershipAnnotations != nil {
				err := json.Unmarshal(e.ProjectMembershipAnnotations, &t.ProjectAnnotations)
				if err != nil {
					return err
				}
			}

			resultMap[e.Tenant.Meta.Id] = t

			return nil
		}
	)

	err := postgres.RunQuery(ctx, s.log, s.db, queryDirectTenantParticipations, input, resultFn)
	if err != nil {
		return nil, err
	}

	includeInherited := true
	if req.IncludeInherited != nil {
		includeInherited = *req.IncludeInherited
	}

	if includeInherited {
		err = postgres.RunQuery(ctx, s.log, s.db, queryInheritedTenantParticipations, input, resultFn)
		if err != nil {
			return nil, err
		}
	}

	for _, t := range resultMap {
		res = append(res, t)
	}

	return &v1.TenantServiceFindParticipatingTenantsResponse{Tenants: res}, nil
}

var (
	queryDirectTenantMembers = sq.Select(
		tenants.JSONField(),
		tenantMembers.JSONField()+"->'meta'->>'annotations' AS tenant_membership_annotations",
	).
		From(tenantMembers.TableName()).
		Join(tenants.TableName() + " ON " + tenants.TableName() + ".id = " + tenantMembers.JSONField() + "->>'member_id'").
		Where(tenantMembers.JSONField() + "->>'tenant_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + tenantMembers.JSONField() + "->> 'namespace', '') = :namespace")

	queryInheritedTenantMembers = sq.Select(
		tenants.JSONField(),
		projects.JSONField(),
	).
		From(projectMembers.TableName()).
		Join(projects.TableName() + " ON " + projects.TableName() + ".id = " + projectMembers.JSONField() + "->>'project_id'").
		Join(tenants.TableName() + " ON " + tenants.TableName() + ".id = " + projectMembers.JSONField() + "->>'tenant_id'").
		Where(projects.JSONField() + "->>'tenant_id' = :tenantId").
		// COALESCE is required to provide an empty string as default value in case the namespace field is not present
		Where("COALESCE(" + projectMembers.JSONField() + "->> 'namespace', '') = :namespace")
)

// ListTenantMembers returns all members of a tenant.
// This includes members which are explicitly participating through a tenant membership but may also
// include memberships, which are inherited by the project memberships (e.g. through project invites).
func (s *tenantService) ListTenantMembers(ctx context.Context, req *v1.TenantServiceListTenantMembersRequest) (*v1.TenantServiceListTenantMembersResponse, error) {
	type result struct {
		Tenant                      *v1.Tenant
		TenantMembershipAnnotations []byte `db:"tenant_membership_annotations"`
		Project                     *v1.Project
	}

	var (
		res       []*v1.TenantWithMembershipAnnotations
		resultMap = map[string]*v1.TenantWithMembershipAnnotations{}

		input = map[string]any{"tenantId": req.TenantId, "namespace": req.Namespace}

		resultFn = func(e result) error {
			t, ok := resultMap[e.Tenant.Meta.Id]
			if !ok {
				t = &v1.TenantWithMembershipAnnotations{
					Tenant: e.Tenant,
				}
			}

			if e.TenantMembershipAnnotations != nil {
				err := json.Unmarshal(e.TenantMembershipAnnotations, &t.TenantAnnotations)
				if err != nil {
					return err
				}
			}

			if e.Project != nil {
				t.ProjectIds = append(t.ProjectIds, e.Project.Meta.Id)
			}

			resultMap[e.Tenant.Meta.Id] = t

			return nil
		}
	)

	err := postgres.RunQuery(ctx, s.log, s.db, queryDirectTenantMembers, input, resultFn)
	if err != nil {
		return nil, err
	}

	includeInherited := true
	if req.IncludeInherited != nil {
		includeInherited = *req.IncludeInherited
	}

	if includeInherited {
		err = postgres.RunQuery(ctx, s.log, s.db, queryInheritedTenantMembers, input, resultFn)
		if err != nil {
			return nil, err
		}
	}

	for _, t := range resultMap {
		res = append(res, t)
	}

	return &v1.TenantServiceListTenantMembersResponse{Tenants: res}, nil
}
