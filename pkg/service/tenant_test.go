package service

import (
	"log/slog"
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/runtime/protoimpl"

	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/test/pg"
)

func TestFindTenant(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
		&v1.Project{},
		&v1.ProjectMember{},
		&v1.Tenant{},
		&v1.TenantMember{},
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t, ves...)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	var (
		tenantStore = postgres.New(log, db, &v1.Tenant{})
		testTenant1 = &v1.Tenant{
			Meta: &v1.Meta{
				Id:   "1",
				Kind: "Tenant",

				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
				Labels: []string{"e", "f"},
			},
			Name:        "tenant-1",
			Description: "tenant 1",
		}
		testTenant2 = &v1.Tenant{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "Tenant",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"c": "d",
					"e": "f",
				},
				Labels: []string{"f", "g", "h"},
			},
			Name:        "tenant-2",
			Description: "tenant 2",
		}

		service = &tenantService{
			db:          db,
			tenantStore: tenantStore,
			log:         log,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.TenantServiceListRequest
		want    *v1.TenantServiceListResponse
		wantErr error
	}{
		{
			name: "find by id",
			req: &v1.TenantServiceListRequest{
				Id: new("1"),
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by id (no results)",
			req: &v1.TenantServiceListRequest{
				Id: new("no-result"),
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by name",
			req: &v1.TenantServiceListRequest{
				Name: new("tenant-2"),
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.TenantServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
				},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #2",
			req: &v1.TenantServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #3",
			req: &v1.TenantServiceListRequest{
				Annotations: map[string]string{
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
					testTenant2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label",
			req: &v1.TenantServiceListRequest{
				Labels: []string{"e"},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #2",
			req: &v1.TenantServiceListRequest{
				Labels: []string{"e", "f"},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #3",
			req: &v1.TenantServiceListRequest{
				Labels: []string{"f"},
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
			},
			want: &v1.TenantServiceListResponse{
				Tenants: []*v1.Tenant{
					testTenant1,
					testTenant2,
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ve := range ves {
				_, err := db.ExecContext(ctx, "TRUNCATE TABLE "+ve.TableName())
				require.NoError(t, err)
			}

			if tt.prepare != nil {
				tt.prepare()
			}

			got, err := service.List(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}
			slices.SortFunc(got.Tenants, func(i, j *v1.Tenant) int {
				if i.Meta.Id < j.Meta.Id {
					return -1
				} else {
					return 1
				}
			})
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreTypes(protoimpl.MessageState{}), cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func Test_tenantService_FindParticipatingProjects(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
		&v1.Project{},
		&v1.ProjectMember{},
		&v1.Tenant{},
		&v1.TenantMember{},
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t, ves...)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	s := &tenantService{
		db:  db,
		log: log,
	}

	var (
		projectStore       = postgres.New(log, db, &v1.Project{})
		tenantMemberStore  = postgres.New(log, db, &v1.TenantMember{})
		projectMemberStore = postgres.New(log, db, &v1.ProjectMember{})
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.TenantServiceFindParticipatingProjectsRequest
		want    *v1.TenantServiceFindParticipatingProjectsResponse
		wantErr error
	}{
		{
			name: "no memberships",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
			},
			want:    &v1.TenantServiceFindParticipatingProjectsResponse{},
			wantErr: nil,
		},
		{
			name: "ignores foreign memberships",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "1",
					TenantId:  "someone else",
				}))
			},
			want:    &v1.TenantServiceFindParticipatingProjectsResponse{},
			wantErr: nil,
		},
		{
			name: "direct membership including 0 inherited",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "1",
					TenantId:  "a",
				}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: []*v1.ProjectWithMembershipAnnotations{{
					Project: &v1.Project{
						Meta: &v1.Meta{
							Kind:       "Project",
							Apiversion: "v1",
							Id:         "1",
						},
					},
					ProjectAnnotations: map[string]string{"role": "admin"},
					TenantAnnotations:  nil,
				}},
			},
			wantErr: nil,
		},
		{
			name: "no direct membership in other namespace",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
				Namespace:        "other",
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "1",
					TenantId:  "a",
				}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: nil,
			},
			wantErr: nil,
		},
		{
			name: "direct membership in a namespace",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
				Namespace:        "a",
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "1",
					TenantId:  "a",
					Namespace: "a",
				}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: []*v1.ProjectWithMembershipAnnotations{{
					Project: &v1.Project{
						Meta: &v1.Meta{
							Kind:       "Project",
							Apiversion: "v1",
							Id:         "1",
						},
					},
					ProjectAnnotations: map[string]string{"role": "admin"},
					TenantAnnotations:  nil,
				}},
			},
			wantErr: nil,
		},
		{
			name: "direct membership excluding inherited",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(false),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}}))
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "2"}, TenantId: "b"}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "1",
					TenantId:  "a",
				}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "2",
					TenantId:  "b",
				}))
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta:     &v1.Meta{Annotations: map[string]string{"role": "editor"}},
					MemberId: "a",
					TenantId: "b",
				}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: []*v1.ProjectWithMembershipAnnotations{{
					Project: &v1.Project{
						Meta: &v1.Meta{
							Kind:       "Project",
							Apiversion: "v1",
							Id:         "1",
						},
					},
					ProjectAnnotations: map[string]string{"role": "admin"},
					TenantAnnotations:  nil,
				}},
			},
			wantErr: nil,
		},
		{
			name: "inherited membership",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "b"}))
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "viewer"}}, TenantId: "b", MemberId: "a"}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: []*v1.ProjectWithMembershipAnnotations{{
					Project: &v1.Project{
						Meta: &v1.Meta{
							Kind:       "Project",
							Apiversion: "v1",
							Id:         "1",
						},
						TenantId: "b",
					},
					ProjectAnnotations: nil,
					TenantAnnotations:  map[string]string{"role": "viewer"},
				}},
			},
			wantErr: nil,
		},
		{
			name: "direct and indirect memberships including inherited",
			req: &v1.TenantServiceFindParticipatingProjectsRequest{
				TenantId:         "req-tenant",
				IncludeInherited: new(true),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, &v1.Project{
					Meta:     &v1.Meta{Id: "direct-1"},
					TenantId: "req-tenant",
				}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "owner"}},
					ProjectId: "direct-1",
					TenantId:  "req-tenant",
				}))
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta:     &v1.Meta{Annotations: map[string]string{"role": "editor"}},
					MemberId: "req-tenant",
					TenantId: "parent",
				}))
				require.NoError(t, projectStore.Create(ctx, &v1.Project{
					Meta:     &v1.Meta{Id: "indirect-2"},
					TenantId: "parent",
				}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "indirect-2",
					TenantId:  "parent",
				}))
			},
			want: &v1.TenantServiceFindParticipatingProjectsResponse{
				Projects: []*v1.ProjectWithMembershipAnnotations{
					{
						Project: &v1.Project{
							Meta: &v1.Meta{
								Kind:       "Project",
								Apiversion: "v1",
								Id:         "direct-1",
							},
							TenantId: "req-tenant",
						},
						ProjectAnnotations: map[string]string{"role": "owner"},
						TenantAnnotations:  nil,
					},
					{
						Project: &v1.Project{
							Meta: &v1.Meta{
								Kind:       "Project",
								Apiversion: "v1",
								Id:         "indirect-2",
							},
							TenantId: "parent",
						},
						ProjectAnnotations: nil,
						TenantAnnotations:  map[string]string{"role": "editor"},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ve := range ves {
				_, err := db.ExecContext(ctx, "TRUNCATE TABLE "+ve.TableName())
				require.NoError(t, err)
			}

			if tt.prepare != nil {
				tt.prepare()
			}

			got, err := s.FindParticipatingProjects(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}
			slices.SortFunc(got.Projects, func(i, j *v1.ProjectWithMembershipAnnotations) int {
				if i.Project.Meta.Id < j.Project.Meta.Id {
					return -1
				} else {
					return 1
				}
			})
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreTypes(protoimpl.MessageState{}), cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func Test_tenantService_FindParticipatingTenants(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
		&v1.Project{},
		&v1.ProjectMember{},
		&v1.Tenant{},
		&v1.TenantMember{},
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t, ves...)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	s := &tenantService{
		db:  db,
		log: log,
	}

	var (
		projectStore       = postgres.New(log, db, &v1.Project{})
		tenantMemberStore  = postgres.New(log, db, &v1.TenantMember{})
		projectMemberStore = postgres.New(log, db, &v1.ProjectMember{})
		tenantStore        = postgres.New(log, db, &v1.Tenant{})
	)

	tests := []struct {
		name    string
		req     *v1.TenantServiceFindParticipatingTenantsRequest
		prepare func()
		want    *v1.TenantServiceFindParticipatingTenantsResponse
		wantErr error
	}{
		{
			name: "no memberships",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {},
			want:    &v1.TenantServiceFindParticipatingTenantsResponse{},
			wantErr: nil,
		},
		{
			name: "ignore foreign memberships",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "a"}})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "c"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "c", TenantId: "b"})
				require.NoError(t, err)
			},
			want:    &v1.TenantServiceFindParticipatingTenantsResponse{},
			wantErr: nil,
		},
		{
			name: "direct membership",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "a", TenantId: "b"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceFindParticipatingTenantsResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "b",
							},
						},
						TenantAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "no direct membership when in different namespace",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
				Namespace:        "other",
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "a", TenantId: "b"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceFindParticipatingTenantsResponse{
				Tenants: nil,
			},
			wantErr: nil,
		},
		{
			name: "direct membership in namespace",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
				Namespace:        "a",
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, Namespace: "a", MemberId: "a", TenantId: "b"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceFindParticipatingTenantsResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "b",
							},
						},
						TenantAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "indirect membership",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "b"})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, ProjectId: "1", TenantId: "a"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceFindParticipatingTenantsResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "b",
							},
						},
						ProjectAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "exclude inherited",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "a",
				IncludeInherited: new(false),
			},
			prepare: func() {
				err := projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "b"})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "b"}})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, ProjectId: "1", TenantId: "a"})
				require.NoError(t, err)
			},
			want:    &v1.TenantServiceFindParticipatingTenantsResponse{},
			wantErr: nil,
		},
		{
			name: "direct and indirect memberships (without interference with other namespaces)",
			req: &v1.TenantServiceFindParticipatingTenantsRequest{
				TenantId:         "req-tnt",
				IncludeInherited: new(true),
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "indirect-tnt"}}))
				require.NoError(t, projectStore.Create(ctx, &v1.Project{
					Meta:     &v1.Meta{Id: "indirect"},
					TenantId: "indirect-tnt",
				}))
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "indirect",
					TenantId:  "req-tnt",
				}))
				// should not interfere:
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "indirect",
					TenantId:  "req-tnt",
					Namespace: "other",
				}))

				require.NoError(t, tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "direct-tnt"}}))
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta:     &v1.Meta{Annotations: map[string]string{"relation": "direct"}},
					TenantId: "direct-tnt",
					MemberId: "req-tnt",
				}))
				// should not interfere:
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta:      &v1.Meta{Annotations: map[string]string{"role": "admin"}},
					ProjectId: "indirect",
					TenantId:  "req-tnt",
					Namespace: "other",
				}))
			},
			want: &v1.TenantServiceFindParticipatingTenantsResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "direct-tnt",
							},
						},
						TenantAnnotations: map[string]string{"relation": "direct"},
					},
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "indirect-tnt",
							},
						},
						ProjectAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ve := range ves {
				_, err := db.ExecContext(ctx, "TRUNCATE TABLE "+ve.TableName())
				require.NoError(t, err)
			}

			if tt.prepare != nil {
				tt.prepare()
			}

			got, err := s.FindParticipatingTenants(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}

			slices.SortFunc(got.Tenants, func(i, j *v1.TenantWithMembershipAnnotations) int {
				if i.Tenant.Meta.Id < j.Tenant.Meta.Id {
					return -1
				} else {
					return 1
				}
			})

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreTypes(protoimpl.MessageState{}), cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func Test_tenantService_ListTenantMembers(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
		&v1.Project{},
		&v1.ProjectMember{},
		&v1.Tenant{},
		&v1.TenantMember{},
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t, ves...)

	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	s := &tenantService{
		db:  db,
		log: log,
	}

	var (
		projectStore       = postgres.New(log, db, &v1.Project{})
		tenantMemberStore  = postgres.New(log, db, &v1.TenantMember{})
		projectMemberStore = postgres.New(log, db, &v1.ProjectMember{})
		tenantStore        = postgres.New(log, db, &v1.Tenant{})
	)

	tests := []struct {
		name    string
		req     *v1.TenantServiceListTenantMembersRequest
		prepare func()
		want    *v1.TenantServiceListTenantMembersResponse
		wantErr error
	}{
		{
			name: "no members",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
			},
			prepare: func() {
			},
			want:    &v1.TenantServiceListTenantMembersResponse{},
			wantErr: nil,
		},
		{
			name: "ignore foreign members",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "acme"}})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "azure"}})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "google"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "azure", TenantId: "google"})
				require.NoError(t, err)
			},
			want:    &v1.TenantServiceListTenantMembersResponse{},
			wantErr: nil,
		},
		{
			name: "direct membership",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "azure"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "azure", TenantId: "acme"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceListTenantMembersResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "azure",
							},
						},
						TenantAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "no direct membership in other namespace",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
				Namespace:        "other",
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "azure"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, MemberId: "azure", TenantId: "acme"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceListTenantMembersResponse{
				Tenants: nil,
			},
			wantErr: nil,
		},
		{
			name: "direct membership in namespace",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
				Namespace:        "a",
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "azure"}})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "admin"}}, Namespace: "a", MemberId: "azure", TenantId: "acme"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceListTenantMembersResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "azure",
							},
						},
						TenantAnnotations: map[string]string{"role": "admin"},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "indirect membership",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "acme"})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "google"}})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "editor"}}, ProjectId: "1", TenantId: "google"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceListTenantMembersResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "google",
							},
						},
						ProjectIds: []string{
							"1",
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "exclude inherited",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "acme",
				IncludeInherited: new(false),
			},
			prepare: func() {
				err := projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "acme"})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "google"}})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"role": "editor"}}, ProjectId: "1", TenantId: "google"})
				require.NoError(t, err)
			},
			want:    &v1.TenantServiceListTenantMembersResponse{},
			wantErr: nil,
		},
		{
			name: "indirect membership in multiple projects",
			req: &v1.TenantServiceListTenantMembersRequest{
				TenantId:         "github",
				IncludeInherited: new(true),
			},
			prepare: func() {
				err := tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "github"}})
				require.NoError(t, err)
				err = tenantStore.Create(ctx, &v1.Tenant{Meta: &v1.Meta{Id: "azure"}})
				require.NoError(t, err)
				err = projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "1"}, TenantId: "github"})
				require.NoError(t, err)
				err = projectStore.Create(ctx, &v1.Project{Meta: &v1.Meta{Id: "2"}, TenantId: "github"})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"project-role": "owner"}}, ProjectId: "1", TenantId: "github"})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"project-role": "owner"}}, ProjectId: "2", TenantId: "github"})
				require.NoError(t, err)
				err = projectMemberStore.Create(ctx, &v1.ProjectMember{Meta: &v1.Meta{Annotations: map[string]string{"project-role": "viewer"}}, ProjectId: "2", TenantId: "azure"})
				require.NoError(t, err)
				err = tenantMemberStore.Create(ctx, &v1.TenantMember{Meta: &v1.Meta{Annotations: map[string]string{"tenant-role": "owner"}}, MemberId: "github", TenantId: "github"})
				require.NoError(t, err)
			},
			want: &v1.TenantServiceListTenantMembersResponse{
				Tenants: []*v1.TenantWithMembershipAnnotations{
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "azure",
							},
						},
						ProjectIds: []string{
							"2",
						},
					},
					{
						Tenant: &v1.Tenant{
							Meta: &v1.Meta{
								Kind:       "Tenant",
								Apiversion: "v1",
								Id:         "github",
							},
						},
						TenantAnnotations: map[string]string{"tenant-role": "owner"},
						ProjectIds: []string{
							"1",
							"2",
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ve := range ves {
				_, err := db.ExecContext(ctx, "TRUNCATE TABLE "+ve.TableName())
				require.NoError(t, err)
			}

			if tt.prepare != nil {
				tt.prepare()
			}

			got, err := s.ListTenantMembers(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}

			slices.SortFunc(got.Tenants, func(i, j *v1.TenantWithMembershipAnnotations) int {
				if i.Tenant.Meta.Id < j.Tenant.Meta.Id {
					return -1
				} else {
					return 1
				}
			})

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
