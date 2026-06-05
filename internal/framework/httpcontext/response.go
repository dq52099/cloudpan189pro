package httpcontext

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
)

type Response struct {
	Msg  string      `json:"msg" example:"success"`
	Code int         `json:"code" example:"200"`
	Data interface{} `json:"data,omitempty"`
}

func (c *Context) Response(httpCode, businessCode int, msg string, data interface{}) {
	if c.prohibitWrite {
		return
	}

	defer func() {
		c.prohibitWrite = true
	}()

	c.JSON(httpCode, Response{
		Msg:  msg,
		Code: businessCode,
		Data: data,
	})
}

func (c *Context) Success(vs ...interface{}) {
	c.Response(http.StatusOK, http.StatusOK, "success", utils.UseSimplify(nil, vs...))
}

func (c *Context) Fail(busErr BusinessError) {
	if c.prohibitWrite {
		return
	}

	defer func() {
		c.prohibitWrite = true
	}()

	if busErr.GetError() != nil {
		c.WithError(busErr.GetError())
	}

	msg := busErr.GetMessage()
	if msg == "" {
		msg = busErr.Error()
	}

	msg = sanitizeResponseMessage(msg)

	c.errMsg = msg

	c.AbortWithStatusJSON(busErr.GetHTTPCode(), Response{
		Code: busErr.GetCode(),
		Msg:  msg,
	})
}

func (c *Context) Unauthorized(messages ...string) *Context {
	c.Fail(unauthorizedBusinessError(messages...))

	return c
}

func (c *Context) Forbidden(messages ...string) *Context {
	c.Fail(forbiddenBusinessError(messages...))

	return c
}

func (c *Context) AbortWithInvalidParams(err error) *Context {
	c.Fail(invalidParamsBusinessError(err))

	return c
}

func (c *Context) WithError(err error) *Context {
	c.errors = append(c.errors, errors.WithStack(err))

	return c
}

func (c *Context) GetErrorMsg() string {
	return c.errMsg
}

func sanitizeResponseMessage(message string) string {
	return sanitizeLoggedText(message)
}
