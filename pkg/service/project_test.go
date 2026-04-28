package service

import (
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/runtime/protoimpl"

	"testing"

	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/datastore/postgres"
)

func TestCreateGetUpdateDeleteProject(t *testing.T) {
	ctx := t.Context()
	client, closer := startTenantApiserverWithPostgres(t, log)
	defer closer()

	t1, err := client.Apiv1().Tenant().Create(ctx, &v1.TenantCreateRequest{Tenant: &v1.Tenant{
		Meta: &v1.Meta{Id: "t1"},
		Name: "t1",
	}})
	require.NoError(t, err)
	p1, err := client.Apiv1().Project().Create(ctx, &v1.ProjectCreateRequest{Project: &v1.Project{
		Meta:     &v1.Meta{Id: "p1"},
		Name:     "p1",
		TenantId: t1.GetTenant().Meta.Id,
	}})
	require.NoError(t, err)

	resp, err := client.Apiv1().Project().Get(ctx, &v1.ProjectGetRequest{Id: p1.Project.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Project)
	assert.Equal(t, p1.Project.Meta.Id, resp.Project.GetMeta().GetId())

	updatedProject := resp.Project
	updatedProject.Description = "Some Project"

	updateresp, err := client.Apiv1().Project().Update(ctx, &v1.ProjectUpdateRequest{Project: updatedProject})
	require.NoError(t, err)
	assert.NotNil(t, updateresp)
	assert.NotNil(t, updateresp.Project)
	assert.Equal(t, "Some Project", updateresp.Project.Description)

	deleteResp, err := client.Apiv1().Project().Delete(ctx, &v1.ProjectDeleteRequest{Id: p1.Project.Meta.Id})
	require.NoError(t, err)
	assert.NotNil(t, deleteResp)
	assert.Equal(t, p1.Project.Meta.Id, deleteResp.Project.GetMeta().GetId())

	getresp, err := client.Apiv1().Project().Get(ctx, &v1.ProjectGetRequest{Id: p1.Project.Meta.Id})
	require.Error(t, err)
	// FIXME this Error should be a notfound
	require.EqualError(t, err, "unknown: rpc error: code = NotFound desc = project with id:p1 not found sql: no rows in result set")
	assert.Nil(t, getresp)
}

func TestFindProject(t *testing.T) {
	ctx := t.Context()
	ves := []api.Entity{
		&v1.Project{},
		&v1.ProjectMember{},
		&v1.Tenant{},
		&v1.TenantMember{},
	}

	container, db, err := startPostgres(ctx, ves...)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
		require.NoError(t, container.Terminate(ctx))
	}()

	var (
		projectStore = postgres.New(log, db, &v1.Project{})
		testProject1 = &v1.Project{
			Meta: &v1.Meta{
				Id:         "1",
				Kind:       "Project",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
				Labels: []string{"e", "f"},
			},
			Name:        "project-1",
			Description: "project 1",
			TenantId:    "tenant-1",
		}
		testProject2 = &v1.Project{
			Meta: &v1.Meta{
				Id:         "2",
				Kind:       "Project",
				Apiversion: "v1",
				Version:    1,
				Annotations: map[string]string{
					"c": "d",
					"e": "f",
				},
				Labels: []string{"f", "g", "h"},
			},
			Name:        "project-2",
			Description: "project 2",
			TenantId:    "tenant-2",
		}

		service = &projectService{
			projectStore: projectStore,
			log:          log,
		}
	)

	tests := []struct {
		name    string
		prepare func()
		req     *v1.ProjectFindRequest
		want    *v1.ProjectListResponse
		wantErr error
	}{
		{
			name: "find by id",
			req: &v1.ProjectFindRequest{
				Id: new("1"),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by id (no results)",
			req: &v1.ProjectFindRequest{
				Id: new("no-result"),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: nil,
			},
			wantErr: nil,
		},
		{
			name: "find by name",
			req: &v1.ProjectFindRequest{
				Name: new("project-2"),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by tenant",
			req: &v1.ProjectFindRequest{
				TenantId: new("tenant-2"),
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation",
			req: &v1.ProjectFindRequest{
				Annotations: map[string]string{
					"a": "b",
				},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #2",
			req: &v1.ProjectFindRequest{
				Annotations: map[string]string{
					"a": "b",
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by annotation #3",
			req: &v1.ProjectFindRequest{
				Annotations: map[string]string{
					"c": "d",
				},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
					testProject2,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label",
			req: &v1.ProjectFindRequest{
				Labels: []string{"e"},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #2",
			req: &v1.ProjectFindRequest{
				Labels: []string{"e", "f"},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
				},
			},
			wantErr: nil,
		},
		{
			name: "find by label #3",
			req: &v1.ProjectFindRequest{
				Labels: []string{"f"},
			},
			prepare: func() {
				require.NoError(t, projectStore.Create(ctx, testProject1))
				require.NoError(t, projectStore.Create(ctx, testProject2))
			},
			want: &v1.ProjectListResponse{
				Projects: []*v1.Project{
					testProject1,
					testProject2,
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

			got, err := service.Find(ctx, tt.req)
			if diff := cmp.Diff(err, tt.wantErr); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
				return
			}
			slices.SortFunc(got.Projects, func(i, j *v1.Project) int {
				if i.Meta.Id < j.Meta.Id {
					return -1
				} else {
					return 1
				}
			})
			if diff := cmp.Diff(tt.want, got, cmpopts.IgnoreTypes(protoimpl.MessageState{}), cmpopts.IgnoreFields(v1.Meta{}, "CreatedTime"), testcommon.IgnoreUnexported()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
