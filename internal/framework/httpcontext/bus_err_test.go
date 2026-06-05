package httpcontext

import (
	"errors"
	"net/http"
	"testing"
)

func TestBusinessErrorModifiersReturnCopies(t *testing.T) {
	template := NewBusinessGenerator(9000).Next("模板错误")

	withError := template.WithError(errors.New("request one failed"))
	withMessage := template.WithMessage("请求二错误")
	withHTTPCode := template.WithHTTPCode(http.StatusNotFound)
	withBusinessCode := template.WithBusinessCode(9010)

	if template.GetError() != nil {
		t.Fatalf("expected template error not to be mutated, got %v", template.GetError())
	}

	if template.GetMessage() != "模板错误" {
		t.Fatalf("expected template message unchanged, got %q", template.GetMessage())
	}

	if template.GetHTTPCode() != http.StatusBadRequest {
		t.Fatalf("expected template HTTP code unchanged, got %d", template.GetHTTPCode())
	}

	if template.GetCode() != 9000 {
		t.Fatalf("expected template business code unchanged, got %d", template.GetCode())
	}

	if withError.GetError() == nil {
		t.Fatal("expected copied error to carry request error")
	}

	if withMessage.GetMessage() != "请求二错误" {
		t.Fatalf("expected copied error message updated, got %q", withMessage.GetMessage())
	}

	if withHTTPCode.GetHTTPCode() != http.StatusNotFound {
		t.Fatalf("expected copied HTTP code updated, got %d", withHTTPCode.GetHTTPCode())
	}

	if withBusinessCode.GetCode() != 9010 {
		t.Fatalf("expected copied business code updated, got %d", withBusinessCode.GetCode())
	}
}

func TestBusinessErrorWithErrorDoesNotLeakPreviousRequestError(t *testing.T) {
	template := NewBusinessGenerator(9100).Next("共享模板")

	first := template.WithError(errors.New("first request"))
	second := template.WithError(errors.New("second request"))

	if template.GetError() != nil {
		t.Fatalf("expected template error not to be mutated, got %v", template.GetError())
	}

	if first.GetError() == nil || second.GetError() == nil {
		t.Fatalf("expected copied errors to keep their request error, first=%v second=%v", first.GetError(), second.GetError())
	}

	if first.GetError().Error() == second.GetError().Error() {
		t.Fatalf("expected request errors to remain isolated, got %q", first.GetError().Error())
	}
}
