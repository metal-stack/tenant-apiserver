package service

import (
	"context"

	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/v"
)

type versionService struct {
}

func NewVersionService() *versionService {
	return &versionService{}
}
func (vs *versionService) Get(context.Context, *v1.VersionServiceGetRequest) (*v1.VersionServiceGetResponse, error) {
	res := &v1.VersionServiceGetResponse{Version: v.Version, Revision: v.Revision, BuildDate: v.BuildDate, GitSha1: v.GitSHA1}
	return res, nil
}
