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
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"
	"github.com/metal-stack/tenant-apiserver/test/pg"
)

func TestCreateGetUpdateDeleteProjectConfig(t *testing.T) {
	ctx := t.Context()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, closer := startTenantApiserverWithPostgres(t, log)
	defer closer()

	t1, err := client.Apiv1().Tenant().Create(ctx, &v1.TenantServiceCreateRequest{Tenant: &v1.Tenant{
		Meta: &v1.Meta{Id: "t1"},
		Name: "t1",
	}})
	require.NoError(t, err)
	p1, err := client.Apiv1().Project().Create(ctx, &v1.ProjectServiceCreateRequest{Project: &v1.Project{
		Meta:     &v1.Meta{Id: "p1"},
		Name:     "p1",
		TenantId: t1.GetTenant().Meta.Id,
	}})
	require.NoError(t, err)

	payload, err := anypb.New(wrapperspb.String("k8s-accounting-payload"))
	require.NoError(t, err)

	pc1, err := client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "k8s-accounting",
		ProjectId: p1.Project.Meta.Id,
		Config:    payload,
	}})
	require.NoError(t, err)
	assert.NotNil(t, pc1.ProjectConfig)
	assert.NotEmpty(t, pc1.ProjectConfig.GetMeta().GetId())

	resp, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: pc1.ProjectConfig.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, resp.ProjectConfig)
	assert.Equal(t, pc1.ProjectConfig.GetMeta().GetId(), resp.ProjectConfig.GetMeta().GetId())
	assert.Equal(t, "k8s-accounting", resp.ProjectConfig.Name)
	assert.Equal(t, p1.Project.Meta.Id, resp.ProjectConfig.ProjectId)

	var value wrapperspb.StringValue
	require.NoError(t, resp.ProjectConfig.Config.UnmarshalTo(&value))
	assert.Equal(t, "k8s-accounting-payload", value.Value)

	pc2, err := client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "iam-client-secret",
		ProjectId: p1.Project.Meta.Id,
	}})
	require.NoError(t, err)
	assert.NotEmpty(t, pc2.ProjectConfig.GetMeta().GetId())

	pc2get, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: pc2.ProjectConfig.GetMeta().GetId()})
	require.NoError(t, err)
	assert.Equal(t, "iam-client-secret", pc2get.ProjectConfig.Name)
	assert.Equal(t, p1.Project.Meta.Id, pc2get.ProjectConfig.ProjectId)

	_, err = client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "orphan",
		ProjectId: "unknown-project",
	}})
	require.Error(t, err)
	require.True(t, errorutil.IsNotFound(err))

	_, err = client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Meta:      &v1.Meta{Id: "pc-set-by-client"},
		Name:      "with-id",
		ProjectId: p1.Project.Meta.Id,
	}})
	require.Error(t, err)
	require.True(t, errorutil.IsInvalidArgument(err))

	updated := resp.ProjectConfig
	updated.Name = "renamed-config"
	updateresp, err := client.Apiv1().ProjectConfig().Update(ctx, &v1.ProjectConfigServiceUpdateRequest{ProjectConfig: updated})
	require.NoError(t, err)
	assert.Equal(t, "renamed-config", updateresp.ProjectConfig.Name)

	movedResp, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: pc1.ProjectConfig.Meta.Id})
	require.NoError(t, err)
	moved := movedResp.ProjectConfig
	moved.ProjectId = "other-project"
	_, err = client.Apiv1().ProjectConfig().Update(ctx, &v1.ProjectConfigServiceUpdateRequest{ProjectConfig: moved})
	require.Error(t, err)
	require.True(t, errorutil.IsInvalidArgument(err))

	deleteResp, err := client.Apiv1().ProjectConfig().Delete(ctx, &v1.ProjectConfigServiceDeleteRequest{Id: pc1.ProjectConfig.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, deleteResp.ProjectConfig)
	assert.Equal(t, pc1.ProjectConfig.GetMeta().GetId(), deleteResp.ProjectConfig.GetMeta().GetId())

	getresp, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: pc1.ProjectConfig.Meta.Id})
	require.Error(t, err)
	require.True(t, errorutil.IsNotFound(err))
	assert.Nil(t, getresp)
}

func TestProjectConfigWithTypedAnyPayload(t *testing.T) {
	ctx := t.Context()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, closer := startTenantApiserverWithPostgres(t, log)
	defer closer()

	t1, err := client.Apiv1().Tenant().Create(ctx, &v1.TenantServiceCreateRequest{Tenant: &v1.Tenant{
		Meta: &v1.Meta{Id: "t1"},
		Name: "t1",
	}})
	require.NoError(t, err)
	p1, err := client.Apiv1().Project().Create(ctx, &v1.ProjectServiceCreateRequest{Project: &v1.Project{
		Meta:     &v1.Meta{Id: "p1"},
		Name:     "p1",
		TenantId: t1.GetTenant().Meta.Id,
	}})
	require.NoError(t, err)

	payload, err := structpb.NewStruct(map[string]any{
		"cluster_name": "metal-stack",
		"replicas":     3,
		"enabled":      true,
	})
	require.NoError(t, err)
	anyPayload, err := anypb.New(payload)
	require.NoError(t, err)
	assert.Equal(t, "type.googleapis.com/google.protobuf.Struct", anyPayload.TypeUrl)
	assert.NotEmpty(t, anyPayload.Value)

	pc, err := client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "k8s-accounting",
		ProjectId: p1.Project.Meta.Id,
		Config:    anyPayload,
	}})
	require.NoError(t, err)

	resp, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: pc.ProjectConfig.GetMeta().GetId()})
	require.NoError(t, err)
	require.NotNil(t, resp.ProjectConfig.Config)

	switch resp.ProjectConfig.Config.GetTypeUrl() {
	case "type.googleapis.com/google.protobuf.Struct":
		var decoded structpb.Struct
		require.NoError(t, resp.ProjectConfig.Config.UnmarshalTo(&decoded))

		assert.Equal(t, "metal-stack", decoded.Fields["cluster_name"].GetStringValue())
		assert.Equal(t, float64(3), decoded.Fields["replicas"].GetNumberValue())
		assert.True(t, decoded.Fields["enabled"].GetBoolValue())
	default:
		t.Fatalf("unexpected type_url %q", resp.ProjectConfig.Config.GetTypeUrl())
	}
}

func TestDeleteProjectDeletesProjectConfigs(t *testing.T) {
	ctx := t.Context()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client, closer := startTenantApiserverWithPostgres(t, log)
	defer closer()

	t1, err := client.Apiv1().Tenant().Create(ctx, &v1.TenantServiceCreateRequest{Tenant: &v1.Tenant{
		Meta: &v1.Meta{Id: "t1"},
		Name: "t1",
	}})
	require.NoError(t, err)
	p1, err := client.Apiv1().Project().Create(ctx, &v1.ProjectServiceCreateRequest{Project: &v1.Project{
		Meta:     &v1.Meta{Id: "p1"},
		Name:     "p1",
		TenantId: t1.GetTenant().Meta.Id,
	}})
	require.NoError(t, err)

	pc1, err := client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "k8s-accounting",
		ProjectId: p1.Project.Meta.Id,
	}})
	require.NoError(t, err)
	pc2, err := client.Apiv1().ProjectConfig().Create(ctx, &v1.ProjectConfigServiceCreateRequest{ProjectConfig: &v1.ProjectConfig{
		Name:      "iam-client-secret",
		ProjectId: p1.Project.Meta.Id,
	}})
	require.NoError(t, err)

	deleteResp, err := client.Apiv1().Project().Delete(ctx, &v1.ProjectServiceDeleteRequest{Id: p1.Project.Meta.Id})
	require.NoError(t, err)
	assert.Equal(t, "p1", deleteResp.Project.GetMeta().GetId())

	for _, id := range []string{pc1.ProjectConfig.GetMeta().GetId(), pc2.ProjectConfig.GetMeta().GetId()} {
		getresp, err := client.Apiv1().ProjectConfig().Get(ctx, &v1.ProjectConfigServiceGetRequest{Id: id})
		require.Error(t, err)
		require.True(t, errorutil.IsNotFound(err))
		assert.Nil(t, getresp)
	}
}

func TestFindProjectConfig(t *testing.T) {
	ctx := t.Context()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	container, db := pg.StartPostgres(log, t)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	var (
		projectConfigStore = postgres.New(log, db, &v1.ProjectConfig{})
		testProjectConfig1 = &v1.ProjectConfig{
			Meta: &v1.Meta{
				Id:         "1",
				Kind:       "ProjectConfig",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
				Labels: []string{"e", "f"},
			},
			Name:      "project-config-1",
			ProjectId: "project-1",
		}
		testProjectConfig2 = &v1.ProjectConfig{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "ProjectConfig",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"c": "d",
					"e": "f",
				},
				Labels: []string{"f", "g", "h"},
			},
			Name:      "project-config-2",
			ProjectId: "project-2",
		}

		service = &projectConfigService{
			projectConfigStore: projectConfigStore,
			log:                log,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.ProjectConfigServiceListRequest
		want    *v1.ProjectConfigServiceListResponse
		wantErr error
	}{
		{
			name: "find by name",
			req: &v1.ProjectConfigServiceListRequest{
				Name: new("project-config-2"),
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by project",
			req: &v1.ProjectConfigServiceListRequest{
				ProjectId: new("project-2"),
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.ProjectConfigServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
				},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #2",
			req: &v1.ProjectConfigServiceListRequest{
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #3",
			req: &v1.ProjectConfigServiceListRequest{
				Annotations: map[string]string{
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
					testProjectConfig2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label",
			req: &v1.ProjectConfigServiceListRequest{
				Labels: []string{"e"},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #2",
			req: &v1.ProjectConfigServiceListRequest{
				Labels: []string{"e", "f"},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #3",
			req: &v1.ProjectConfigServiceListRequest{
				Labels: []string{"f"},
			},
			prepare: func() {
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig1))
				require.NoError(t, projectConfigStore.Create(ctx, testProjectConfig2))
			},
			want: &v1.ProjectConfigServiceListResponse{
				ProjectConfig: []*v1.ProjectConfig{
					testProjectConfig1,
					testProjectConfig2,
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
			slices.SortFunc(got.ProjectConfig, func(i, j *v1.ProjectConfig) int {
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
