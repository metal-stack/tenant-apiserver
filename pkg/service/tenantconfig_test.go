package service

import (
	"log/slog"
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/metal-stack/tenant-api/go/tenant/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/runtime/protoimpl"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
	"github.com/metal-stack/tenant-apiserver/test/pg"
)

func TestCreateGetUpdateDeleteTenantConfig(t *testing.T) {
	ctx := t.Context()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, closer := startTenantApiserverWithPostgres(t, log)
	defer closer()

	t1, err := client.Apiv1().Tenant().Create(ctx, &v1.TenantServiceCreateRequest{Tenant: &v1.Tenant{
		Meta: &v1.Meta{Id: "t1"},
		Name: "t1",
	}})
	require.NoError(t, err)

	payload, err := anypb.New(wrapperspb.String("k8s-accounting-payload"))
	require.NoError(t, err)

	tc1, err := client.Apiv1().TenantConfig().Create(ctx, &v1.TenantConfigServiceCreateRequest{TenantConfig: &v1.TenantConfig{
		Meta:     &v1.Meta{Id: "tc1"},
		Name:     "k8s-accounting",
		TenantId: t1.GetTenant().Meta.Id,
		Config:   payload,
	}})
	require.NoError(t, err)
	assert.NotNil(t, tc1.TenantConfig)
	assert.Equal(t, "tc1", tc1.TenantConfig.GetMeta().GetId())

	resp, err := client.Apiv1().TenantConfig().Get(ctx, &v1.TenantConfigServiceGetRequest{Id: tc1.TenantConfig.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, resp.TenantConfig)
	assert.Equal(t, "tc1", resp.TenantConfig.GetMeta().GetId())
	assert.Equal(t, "k8s-accounting", resp.TenantConfig.Name)
	assert.Equal(t, t1.GetTenant().Meta.Id, resp.TenantConfig.TenantId)

	// the arbitrary config payload must survive the round trip
	var value wrapperspb.StringValue
	require.NoError(t, resp.TenantConfig.Config.UnmarshalTo(&value))
	assert.Equal(t, "k8s-accounting-payload", value.Value)

	// creating without meta must generate an id
	tc2, err := client.Apiv1().TenantConfig().Create(ctx, &v1.TenantConfigServiceCreateRequest{TenantConfig: &v1.TenantConfig{
		Name:     "iam-client-secret",
		TenantId: t1.GetTenant().Meta.Id,
	}})
	require.NoError(t, err)
	assert.NotEmpty(t, tc2.TenantConfig.GetMeta().GetId())

	tc2get, err := client.Apiv1().TenantConfig().Get(ctx, &v1.TenantConfigServiceGetRequest{Id: tc2.TenantConfig.GetMeta().GetId()})
	require.NoError(t, err)
	assert.Equal(t, "iam-client-secret", tc2get.TenantConfig.Name)
	assert.Equal(t, t1.GetTenant().Meta.Id, tc2get.TenantConfig.TenantId)

	// creating for an unknown tenant must fail with not found
	_, err = client.Apiv1().TenantConfig().Create(ctx, &v1.TenantConfigServiceCreateRequest{TenantConfig: &v1.TenantConfig{
		Meta:     &v1.Meta{Id: "tc3"},
		Name:     "orphan",
		TenantId: "unknown-tenant",
	}})
	require.Error(t, err)
	require.True(t, errorutil.IsNotFound(err))

	// updating the name is allowed
	updated := resp.TenantConfig
	updated.Name = "renamed-config"
	updateresp, err := client.Apiv1().TenantConfig().Update(ctx, &v1.TenantConfigServiceUpdateRequest{TenantConfig: updated})
	require.NoError(t, err)
	assert.Equal(t, "renamed-config", updateresp.TenantConfig.Name)

	// moving the config to another tenant is not allowed
	movedResp, err := client.Apiv1().TenantConfig().Get(ctx, &v1.TenantConfigServiceGetRequest{Id: tc1.TenantConfig.Meta.Id})
	require.NoError(t, err)
	moved := movedResp.TenantConfig
	moved.TenantId = "other-tenant"
	_, err = client.Apiv1().TenantConfig().Update(ctx, &v1.TenantConfigServiceUpdateRequest{TenantConfig: moved})
	require.Error(t, err)
	require.True(t, errorutil.IsInvalidArgument(err))

	deleteResp, err := client.Apiv1().TenantConfig().Delete(ctx, &v1.TenantConfigServiceDeleteRequest{Id: tc1.TenantConfig.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, deleteResp.TenantConfig)
	assert.Equal(t, "tc1", deleteResp.TenantConfig.GetMeta().GetId())

	getresp, err := client.Apiv1().TenantConfig().Get(ctx, &v1.TenantConfigServiceGetRequest{Id: tc1.TenantConfig.Meta.Id})
	require.Error(t, err)
	require.True(t, errorutil.IsNotFound(err))
	assert.Nil(t, getresp)
}

func TestFindTenantConfig(t *testing.T) {
	ctx := t.Context()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	var (
		tenantConfigStore = postgres.New(log, db, &v1.TenantConfig{})
		testTenantConfig1 = &v1.TenantConfig{
			Meta: &v1.Meta{
				Id:         "1",
				Kind:       "TenantConfig",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
				Labels: []string{"e", "f"},
			},
			Name:     "tenant-config-1",
			TenantId: "tenant-1",
		}
		testTenantConfig2 = &v1.TenantConfig{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "TenantConfig",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"c": "d",
					"e": "f",
				},
				Labels: []string{"f", "g", "h"},
			},
			Name:     "tenant-config-2",
			TenantId: "tenant-2",
		}

		service = &tenantConfigService{
			tenantConfigStore: tenantConfigStore,
			log:               log,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.TenantConfigServiceListRequest
		want    *v1.TenantConfigServiceListResponse
		wantErr error
	}{
		{
			name: "find by id",
			req: &v1.TenantConfigServiceListRequest{
				Id: new("1"),
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by id (no results)",
			req: &v1.TenantConfigServiceListRequest{
				Id: new("no-result"),
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by name",
			req: &v1.TenantConfigServiceListRequest{
				Name: new("tenant-config-2"),
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by tenant",
			req: &v1.TenantConfigServiceListRequest{
				TenantId: new("tenant-2"),
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.TenantConfigServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
				},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #2",
			req: &v1.TenantConfigServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #3",
			req: &v1.TenantConfigServiceListRequest{
				Annotations: map[string]string{
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
					testTenantConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label",
			req: &v1.TenantConfigServiceListRequest{
				Labels: []string{"e"},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #2",
			req: &v1.TenantConfigServiceListRequest{
				Labels: []string{"e", "f"},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #3",
			req: &v1.TenantConfigServiceListRequest{
				Labels: []string{"f"},
			},
			prepare: func() {
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig1))
				require.NoError(t, tenantConfigStore.Create(ctx, testTenantConfig2))
			},
			want: &v1.TenantConfigServiceListResponse{
				TenantConfig: []*v1.TenantConfig{
					testTenantConfig1,
					testTenantConfig2,
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ve := range pg.AllEntities() {
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
			slices.SortFunc(got.TenantConfig, func(i, j *v1.TenantConfig) int {
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
