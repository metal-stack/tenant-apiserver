package service

import (
	"log/slog"
	"os"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"

	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
	"github.com/metal-stack/tenant-apiserver/test/pg"
)

func TestFindProjectMember(t *testing.T) {
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
		projectMemberStore = postgres.New(log, db, &v1.ProjectMember{})
		projectStore       = postgres.New(log, db, &v1.Project{})
		tenantStore        = postgres.New(log, db, &v1.Tenant{})

		testTenant1 = &v1.Tenant{
			Meta: &v1.Meta{
				Id:         "tenant-1",
				Kind:       "Tenant",
				Apiversion: "v1",
				Version:    1,
			},
			Name:        "tenant 1",
			Description: "test tenant 1",
		}
		testTenant2 = &v1.Tenant{
			Meta: &v1.Meta{
				Id:         "tenant-2",
				Kind:       "Tenant",
				Apiversion: "v1",
				Version:    1,
			},
			Name:        "tenant 2",
			Description: "test tenant 2",
		}
		testProject1 = &v1.Project{
			Meta: &v1.Meta{
				Id:         "project-1",
				Kind:       "Project",
				Apiversion: "v1",
				Version:    1,
			},
			Name:        "project 1",
			Description: "test project 1",
			TenantId:    "tenant-1",
		}
		testProjectMember1 = &v1.ProjectMember{
			Meta: &v1.Meta{
				Id:         "1",
				Kind:       "ProjectMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "owner",
				},
				Labels: []string{"a", "b"},
			},
			ProjectId: "project-1",
			TenantId:  "tenant-1",
			Namespace: "a",
		}
		testProjectMember2 = &v1.ProjectMember{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "ProjectMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "viewer",
				},
				Labels: []string{"c", "d"},
			},
			ProjectId: "project-1",
			TenantId:  "tenant-2",
			Namespace: "a",
		}
		testProjectMember3 = &v1.ProjectMember{
			Meta: &v1.Meta{
				Id:         "3",
				Kind:       "ProjectMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "owner",
				},
				Labels: []string{"e", "f"},
			},
			ProjectId: "project-2",
			TenantId:  "tenant-2",
			Namespace: "a",
		}
		testProjectMember4 = &v1.ProjectMember{
			Meta: &v1.Meta{
				Id:         "4",
				Kind:       "ProjectMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "owner",
				},
			},
			ProjectId: "project-2",
			TenantId:  "tenant-2",
			Namespace: "",
		}

		service = &projectMemberService{
			log:                log,
			projectMemberStore: projectMemberStore,
			tenantStore:        tenantStore,
			projectStore:       projectStore,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.ProjectMemberServiceListRequest
		want    *v1.ProjectMemberServiceListResponse
		wantErr error
	}{
		{
			name: "find by project",
			req: &v1.ProjectMemberServiceListRequest{
				ProjectId: new("project-1"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember2))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember3))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember4))
			},
			want: &v1.ProjectMemberServiceListResponse{
				ProjectMembers: []*v1.ProjectMember{
					testProjectMember1,
					testProjectMember2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by project id (no results) #1",
			req: &v1.ProjectMemberServiceListRequest{
				ProjectId: new("no-result"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember2))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember3))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember4))
			},
			want: &v1.ProjectMemberServiceListResponse{
				ProjectMembers: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by project id (no results) #2",
			req: &v1.ProjectMemberServiceListRequest{
				ProjectId: new("project-1"),
				Namespace: "wrong-namespace",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember2))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember3))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember4))
			},
			want: &v1.ProjectMemberServiceListResponse{
				ProjectMembers: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by tenant",
			req: &v1.ProjectMemberServiceListRequest{
				TenantId:  new("tenant-2"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember2))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember3))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember4))
			},
			want: &v1.ProjectMemberServiceListResponse{
				ProjectMembers: []*v1.ProjectMember{
					testProjectMember2,
					testProjectMember3,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.ProjectMemberServiceListRequest{
				Annotations: map[string]string{"role": "owner"},
				Namespace:   "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember1))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember2))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember3))
				require.NoError(t, projectMemberStore.Create(ctx, testProjectMember4))
			},
			want: &v1.ProjectMemberServiceListResponse{
				ProjectMembers: []*v1.ProjectMember{
					testProjectMember1,
					testProjectMember3,
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

			slices.SortFunc(got.ProjectMembers, func(i, j *v1.ProjectMember) int {
				if i.Meta.Id < j.Meta.Id {
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

func TestUpdateProjectMember(t *testing.T) {
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
		projectMemberStore = postgres.New(log, db, &v1.ProjectMember{})
		projectStore       = postgres.New(log, db, &v1.Project{})
		tenantStore        = postgres.New(log, db, &v1.Tenant{})

		service = &projectMemberService{
			log:                log,
			projectMemberStore: projectMemberStore,
			tenantStore:        tenantStore,
			projectStore:       projectStore,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.ProjectMemberServiceUpdateRequest
		want    *v1.ProjectMemberServiceUpdateResponse
		wantErr error
	}{
		{
			name: "update mutable fields",
			req: &v1.ProjectMemberServiceUpdateRequest{
				ProjectMember: &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
						Annotations: map[string]string{
							"role": "owner",
						},
						Labels: []string{"a", "b"},
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "ProjectMember",
						Apiversion: "v1",
						Version:    1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want: &v1.ProjectMemberServiceUpdateResponse{
				ProjectMember: &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "ProjectMember",
						Apiversion: "v1",
						Version:    2,
						Annotations: map[string]string{
							"role": "owner",
						},
						Labels: []string{"a", "b"},
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				},
			},
			wantErr: nil,
		},
		{
			name: "unable to update namespace",
			req: &v1.ProjectMemberServiceUpdateRequest{
				ProjectMember: &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "b",
				},
			},
			prepare: func() {
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "ProjectMember",
						Apiversion: "v1",
						Version:    1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the namespace of a project member is not allowed"),
		},
		{
			name: "unable to update project",
			req: &v1.ProjectMemberServiceUpdateRequest{
				ProjectMember: &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					ProjectId: "project-2",
					TenantId:  "tenant-1",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "ProjectMember",
						Apiversion: "v1",
						Version:    1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the project id of a project member is not allowed"),
		},
		{
			name: "unable to update tenant",
			req: &v1.ProjectMemberServiceUpdateRequest{
				ProjectMember: &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-2",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, projectMemberStore.Create(ctx, &v1.ProjectMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "ProjectMember",
						Apiversion: "v1",
						Version:    1,
					},
					ProjectId: "project-1",
					TenantId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the tenant id of a project member is not allowed"),
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

			got, err := service.Update(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr, errorutil.ConnectErrorComparer()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}

			if err == nil {
				assert.NotNil(t, got.ProjectMember.Meta.UpdatedTime)
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime", "UpdatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
