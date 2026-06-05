package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/repository/models"
	userSvi "github.com/xxcheng123/cloudpan189-share/internal/services/user"
	usergroupSvi "github.com/xxcheng123/cloudpan189-share/internal/services/usergroup"
)

func TestListSkipsDefaultGroupBeforeBatchQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userSvc := &listUserServiceStub{
		users: []*models.User{
			{ID: 1, Username: "default", GroupID: 0},
			{ID: 2, Username: "vip-user", GroupID: 2},
		},
		count: 2,
	}
	groupSvc := &listUserGroupServiceStub{
		groups: []*models.UserGroup{
			{ID: 2, Name: "VIP"},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(nil)
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, groupSvc, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if len(groupSvc.batchIDs) != 1 || groupSvc.batchIDs[0] != 2 {
		t.Fatalf("expected BatchQuery ids [2], got %#v", groupSvc.batchIDs)
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID        int64  `json:"id"`
				GroupID   int64  `json:"groupId"`
				GroupName string `json:"groupName"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("expected business code 200, got %d", response.Code)
	}

	if response.Data.Total != 2 || len(response.Data.Data) != 2 {
		t.Fatalf("expected two users in response, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	if response.Data.Data[0].GroupName != "默认用户组" {
		t.Fatalf("expected default group name, got %q", response.Data.Data[0].GroupName)
	}

	if response.Data.Data[1].GroupName != "VIP" {
		t.Fatalf("expected VIP group name, got %q", response.Data.Data[1].GroupName)
	}
}

func TestListSkipsNilUsersAndGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userSvc := &listUserServiceStub{
		users: []*models.User{
			nil,
			{ID: 1, Username: "default", GroupID: 0},
			{ID: 2, Username: "vip-user", GroupID: 2},
		},
	}
	groupSvc := &listUserGroupServiceStub{
		groups: []*models.UserGroup{
			nil,
			{ID: 2, Name: "VIP"},
		},
	}

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(nil)
	router.GET("/list", wrapper.Wrap(NewHandler(userSvc, groupSvc, nil).List()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/list?noPaginate=true", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	if len(groupSvc.batchIDs) != 1 || groupSvc.batchIDs[0] != 2 {
		t.Fatalf("expected BatchQuery ids [2], got %#v", groupSvc.batchIDs)
	}

	var response struct {
		Code int `json:"code"`
		Data struct {
			Total int64 `json:"total"`
			Data  []struct {
				ID        int64  `json:"id"`
				GroupName string `json:"groupName"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Total != 2 || len(response.Data.Data) != 2 {
		t.Fatalf("expected two non-nil users, got total=%d len=%d", response.Data.Total, len(response.Data.Data))
	}

	if response.Data.Data[0].ID != 1 || response.Data.Data[0].GroupName != "默认用户组" {
		t.Fatalf("unexpected first user: %+v", response.Data.Data[0])
	}

	if response.Data.Data[1].ID != 2 || response.Data.Data[1].GroupName != "VIP" {
		t.Fatalf("unexpected second user: %+v", response.Data.Data[1])
	}
}

type listUserServiceStub struct {
	users []*models.User
	count int64
}

func (s *listUserServiceStub) Add(context.Context, *userSvi.AddRequest, ...userSvi.AddOptionFunc) (*userSvi.AddResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserServiceStub) Del(context.Context, *userSvi.DelRequest) error {
	return errors.New("not implemented")
}

func (s *listUserServiceStub) Query(context.Context, int64) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserServiceStub) QueryByUsername(context.Context, string) (*models.User, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserServiceStub) Update(context.Context, int64, ...utils.Field) error {
	return errors.New("not implemented")
}

func (s *listUserServiceStub) List(context.Context, *userSvi.ListRequest) ([]*models.User, error) {
	return s.users, nil
}

func (s *listUserServiceStub) Count(context.Context, *userSvi.ListRequest) (int64, error) {
	return s.count, nil
}

func (s *listUserServiceStub) ModifyPass(context.Context, int64, string) error {
	return errors.New("not implemented")
}

func (s *listUserServiceStub) BindGroup(context.Context, *userSvi.BindGroupRequest) error {
	return errors.New("not implemented")
}

func (s *listUserServiceStub) ParseAccessToken(string) (int64, string, int, error) {
	return 0, "", 0, errors.New("not implemented")
}

func (s *listUserServiceStub) GenerateAccessToken(int64, string, int) (string, error) {
	return "", errors.New("not implemented")
}

func (s *listUserServiceStub) GenerateRefreshToken(int64, string, int) (string, error) {
	return "", errors.New("not implemented")
}

func (s *listUserServiceStub) ParseRefreshToken(string) (int64, string, int, error) {
	return 0, "", 0, errors.New("not implemented")
}

func (s *listUserServiceStub) GetExpire() int64 {
	return 0
}

type listUserGroupServiceStub struct {
	groups   []*models.UserGroup
	batchIDs []int64
}

func (s *listUserGroupServiceStub) Add(context.Context, *usergroupSvi.AddRequest) (*usergroupSvi.AddResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserGroupServiceStub) Delete(context.Context, *usergroupSvi.DeleteRequest) error {
	return errors.New("not implemented")
}

func (s *listUserGroupServiceStub) ModifyName(context.Context, *usergroupSvi.ModifyNameRequest) error {
	return errors.New("not implemented")
}

func (s *listUserGroupServiceStub) List(context.Context, *usergroupSvi.ListRequest) ([]*models.UserGroup, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserGroupServiceStub) Count(context.Context, *usergroupSvi.ListRequest) (int64, error) {
	return 0, errors.New("not implemented")
}

func (s *listUserGroupServiceStub) Query(context.Context, int64) (*models.UserGroup, error) {
	return nil, errors.New("not implemented")
}

func (s *listUserGroupServiceStub) BatchQuery(_ context.Context, idList []int64) ([]*models.UserGroup, error) {
	s.batchIDs = append([]int64(nil), idList...)
	for _, id := range idList {
		if id == 0 {
			return nil, errors.New("default group id must not be queried")
		}
	}

	return s.groups, nil
}
