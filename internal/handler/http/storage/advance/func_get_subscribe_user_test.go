package advance

import (
	stdctx "context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	appContext "github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
	cloudbridgeSvi "github.com/xxcheng123/cloudpan189-share/internal/services/cloudbridge"
	"go.uber.org/zap"
)

type mockGetSubscribeUserCloudBridge struct {
	cloudbridgeSvi.Service
	infoUsers []string
	listUsers []string
	allUsers  []string
	nilInfo   bool
}

func (m *mockGetSubscribeUserCloudBridge) GetSubscribeUserInfo(ctx appContext.Context, userId string) (*cloudbridgeSvi.SubscribeUserInfo, error) {
	m.infoUsers = append(m.infoUsers, userId)

	if m.nilInfo {
		return nil, nil
	}

	return &cloudbridgeSvi.SubscribeUserInfo{UserId: userId, Name: "订阅号_" + userId}, nil
}

func (m *mockGetSubscribeUserCloudBridge) GetSubscribeUserShareResource(ctx appContext.Context, userId string, opts ...cloudbridgeSvi.SubscribeUserShareResourceOptionFunc) ([]*cloudbridgeSvi.ShareResourceInfo, int64, error) {
	m.listUsers = append(m.listUsers, userId)

	return []*cloudbridgeSvi.ShareResourceInfo{}, 0, nil
}

func (m *mockGetSubscribeUserCloudBridge) GetSubscribeUserShareResourceAll(ctx appContext.Context, userId string) ([]*cloudbridgeSvi.ShareResourceInfo, int64, error) {
	m.allUsers = append(m.allUsers, userId)

	return []*cloudbridgeSvi.ShareResourceInfo{}, 0, nil
}

func TestGetSubscribeUserNormalizesSubscribeLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetSubscribeUserCloudBridge{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user", wrapper.Wrap(NewHandler(cloudBridge, nil).GetSubscribeUser()))

	values := url.Values{}
	values.Set("subscribeUser", "https://content.21cn.com/h5/subscrip/?uuid=encoded%2Duser%5F9。")
	values.Set("currentPage", "1")
	values.Set("pageSize", "30")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user?"+values.Encode(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.infoUsers, []string{"encoded-user_9"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized info lookup %v, got %v", want, got)
	}

	if got, want := cloudBridge.listUsers, []string{"encoded-user_9"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized resource lookup %v, got %v", want, got)
	}
}

func TestGetSubscribeUserRejectsLookalikeSubscribeLinkBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetSubscribeUserCloudBridge{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user", wrapper.Wrap(NewHandler(cloudBridge, nil).GetSubscribeUser()))

	values := url.Values{}
	values.Set("subscribeUser", "note https://content.21cn.com.evil.test/h5/subscrip/?uuid=bad-user")
	values.Set("currentPage", "1")
	values.Set("pageSize", "30")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user?"+values.Encode(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if len(cloudBridge.infoUsers) != 0 || len(cloudBridge.listUsers) != 0 {
		t.Fatalf("expected remote subscribe lookup not to be called, got info=%v list=%v", cloudBridge.infoUsers, cloudBridge.listUsers)
	}
}

func TestGetSubscribeUserReturnsQueryErrorWhenInfoIsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetSubscribeUserCloudBridge{nilInfo: true}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user", wrapper.Wrap(NewHandler(cloudBridge, nil).GetSubscribeUser()))

	values := url.Values{}
	values.Set("subscribeUser", "valid-user")
	values.Set("currentPage", "1")
	values.Set("pageSize", "30")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQuerySubscribeUserError)

	if len(cloudBridge.listUsers) != 0 {
		t.Fatalf("expected resource lookup not to run without user info, got %v", cloudBridge.listUsers)
	}
}

func TestGetSubscribeUserReturnsQueryErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user", wrapper.Wrap(NewHandler(nil, nil).GetSubscribeUser()))

	values := url.Values{}
	values.Set("subscribeUser", "valid-user")
	values.Set("currentPage", "1")
	values.Set("pageSize", "30")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQuerySubscribeUserError)
}

func TestGetSubscribeUserAllNormalizesSubscribeLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetSubscribeUserCloudBridge{}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user_all", wrapper.Wrap(NewHandler(cloudBridge, nil).GetSubscribeUserAll()))

	values := url.Values{}
	values.Set("subscribeUser", "123 https://content.21cn.com/h5/subscrip/?uuid=number-prefix-user")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user_all?"+values.Encode(), nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	if got, want := cloudBridge.infoUsers, []string{"number-prefix-user"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized info lookup %v, got %v", want, got)
	}

	if got, want := cloudBridge.allUsers, []string{"number-prefix-user"}; !stringSlicesEqual(got, want) {
		t.Fatalf("expected normalized all-resource lookup %v, got %v", want, got)
	}
}

func TestGetSubscribeUserAllReturnsQueryErrorWhenInfoIsNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cloudBridge := &mockGetSubscribeUserCloudBridge{nilInfo: true}
	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user_all", wrapper.Wrap(NewHandler(cloudBridge, nil).GetSubscribeUserAll()))

	values := url.Values{}
	values.Set("subscribeUser", "valid-user")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user_all?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQuerySubscribeUserError)

	if len(cloudBridge.allUsers) != 0 {
		t.Fatalf("expected all-resource lookup not to run without user info, got %v", cloudBridge.allUsers)
	}
}

func TestGetSubscribeUserAllReturnsQueryErrorWhenCloudBridgeServiceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	wrapper := httpcontext.NewHandlerFuncWrapper(zap.NewNop())
	router.GET("/get_subscribe_user_all", wrapper.Wrap(NewHandler(nil, nil).GetSubscribeUserAll()))

	values := url.Values{}
	values.Set("subscribeUser", "valid-user")

	req := httptest.NewRequestWithContext(stdctx.Background(), http.MethodGet, "/get_subscribe_user_all?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assertAdvanceHTTPError(t, recorder, http.StatusBadRequest, codeStorageAdvanceQuerySubscribeUserError)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
