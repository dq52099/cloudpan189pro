package file

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/xxcheng123/cloudpan189-share/internal/framework/httpcontext"
)

func assertFileHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedErr httpcontext.BusinessError) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected HTTP %d, got %d body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}

	var response httpcontext.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Code != expectedErr.GetCode() {
		t.Fatalf("expected business code %d, got %d", expectedErr.GetCode(), response.Code)
	}
}
