package service

import (
	"testing"

	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/v"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetVersion(t *testing.T) {

	vs := &versionService{}
	ctx := t.Context()

	expected := v1.VersionServiceGetResponse{Version: v.Version, Revision: v.Revision, BuildDate: v.BuildDate, GitSha1: v.GitSHA1}

	result, err := vs.Get(ctx, &v1.VersionServiceGetRequest{})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expected.Version, result.Version)
	assert.Equal(t, expected.Revision, result.Revision)
	assert.Equal(t, expected.BuildDate, result.BuildDate)
	assert.Equal(t, expected.GitSha1, result.GitSha1)
}
