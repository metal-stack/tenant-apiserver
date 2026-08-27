package service

import (
	"log/slog"
	"os"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"

	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
	"github.com/metal-stack/tenant-apiserver/test/pg"
)

func TestFindTenantMember(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
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
		tenantMemberStore = postgres.New(log, db, &v1.TenantMember{})
		tenantStore       = postgres.New(log, db, &v1.Tenant{})

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
		testTenantMember1 = &v1.TenantMember{
			Meta: &v1.Meta{
				Id:         "1",
				Kind:       "TenantMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "owner",
				},
				Labels: []string{"a", "b"},
			},
			TenantId:  "tenant-1",
			MemberId:  "tenant-1",
			Namespace: "a",
		}
		testTenantMember2 = &v1.TenantMember{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "TenantMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "owner",
				},
				Labels: []string{"c", "d"},
			},
			TenantId:  "tenant-2",
			MemberId:  "tenant-2",
			Namespace: "a",
		}
		testTenantMember3 = &v1.TenantMember{
			Meta: &v1.Meta{
				Id:         "3",
				Kind:       "TenantMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "editor",
				},
				Labels: []string{"e", "f"},
			},
			TenantId:  "tenant-1",
			MemberId:  "tenant-2",
			Namespace: "a",
		}
		testTenantMember4 = &v1.TenantMember{
			Meta: &v1.Meta{
				Id:         "4",
				Kind:       "TenantMember",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"role": "editor",
				},
				Labels: []string{"e", "f"},
			},
			TenantId:  "tenant-1",
			MemberId:  "tenant-2",
			Namespace: "",
		}

		service = &tenantMemberService{
			log:               log,
			tenantMemberStore: tenantMemberStore,
			tenantStore:       tenantStore,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.TenantMemberServiceListRequest
		want    *v1.TenantMemberServiceListResponse
		wantErr error
	}{
		{
			name: "find by tenant",
			req: &v1.TenantMemberServiceListRequest{
				TenantId:  new("tenant-1"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember1))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember3))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember4))
			},
			want: &v1.TenantMemberServiceListResponse{
				TenantMembers: []*v1.TenantMember{
					testTenantMember1,
					testTenantMember3,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by tenant id (no results) #1",
			req: &v1.TenantMemberServiceListRequest{
				TenantId:  new("no-result"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember1))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember3))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember4))
			},
			want: &v1.TenantMemberServiceListResponse{
				TenantMembers: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by tenant id (no results) #2",
			req: &v1.TenantMemberServiceListRequest{
				TenantId:  new("tenant-1"),
				Namespace: "wrong-namespace",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember1))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember3))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember4))
			},
			want: &v1.TenantMemberServiceListResponse{
				TenantMembers: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by tenant",
			req: &v1.TenantMemberServiceListRequest{
				TenantId:  new("tenant-2"),
				Namespace: "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember1))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember3))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember4))
			},
			want: &v1.TenantMemberServiceListResponse{
				TenantMembers: []*v1.TenantMember{
					testTenantMember2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.TenantMemberServiceListRequest{
				Annotations: map[string]string{"role": "owner"},
				Namespace:   "a",
			},
			prepare: func() {
				require.NoError(t, tenantStore.Create(ctx, testTenant1))
				require.NoError(t, tenantStore.Create(ctx, testTenant2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember1))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember2))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember3))
				require.NoError(t, tenantMemberStore.Create(ctx, testTenantMember4))
			},
			want: &v1.TenantMemberServiceListResponse{
				TenantMembers: []*v1.TenantMember{
					testTenantMember1,
					testTenantMember2,
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

			slices.SortFunc(got.TenantMembers, func(i, j *v1.TenantMember) int {
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

func TestUpdateTenantMember(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
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
		tenantMemberStore = postgres.New(log, db, &v1.TenantMember{})
		tenantStore       = postgres.New(log, db, &v1.Tenant{})

		service = &tenantMemberService{
			log:               log,
			tenantMemberStore: tenantMemberStore,
			tenantStore:       tenantStore,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.TenantMemberServiceUpdateRequest
		want    *v1.TenantMemberServiceUpdateResponse
		wantErr error
	}{
		{
			name: "update mutable fields",
			req: &v1.TenantMemberServiceUpdateRequest{
				TenantMember: &v1.TenantMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
						Annotations: map[string]string{
							"role": "owner",
						},
						Labels: []string{"a", "b"},
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "TenantMember",
						Apiversion: "v1",
						Version:    1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want: &v1.TenantMemberServiceUpdateResponse{
				TenantMember: &v1.TenantMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "TenantMember",
						Apiversion: "v1",
						Version:    2,
						Annotations: map[string]string{
							"role": "owner",
						},
						Labels: []string{"a", "b"},
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				},
			},
			wantErr: nil,
		},
		{
			name: "unable to update namespace",
			req: &v1.TenantMemberServiceUpdateRequest{
				TenantMember: &v1.TenantMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "b",
				},
			},
			prepare: func() {
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "TenantMember",
						Apiversion: "v1",
						Version:    1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the namespace of a tenant member is not allowed"),
		},
		{
			name: "unable to update tenant id",
			req: &v1.TenantMemberServiceUpdateRequest{
				TenantMember: &v1.TenantMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					TenantId:  "tenant-2",
					MemberId:  "tenant-1",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "TenantMember",
						Apiversion: "v1",
						Version:    1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the tenant id of a tenant member is not allowed"),
		},
		{
			name: "unable to update member id",
			req: &v1.TenantMemberServiceUpdateRequest{
				TenantMember: &v1.TenantMember{
					Meta: &v1.Meta{
						Id:      "1",
						Version: 1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-2",
					Namespace: "a",
				},
			},
			prepare: func() {
				require.NoError(t, tenantMemberStore.Create(ctx, &v1.TenantMember{
					Meta: &v1.Meta{
						Id:         "1",
						Kind:       "TenantMember",
						Apiversion: "v1",
						Version:    1,
					},
					TenantId:  "tenant-1",
					MemberId:  "tenant-1",
					Namespace: "a",
				}))
			},
			want:    nil,
			wantErr: errorutil.InvalidArgument("updating the member id of a tenant member is not allowed"),
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
				assert.NotNil(t, got.TenantMember.Meta.UpdatedTime)
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime", "UpdatedTime"), IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
