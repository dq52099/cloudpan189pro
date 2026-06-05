package verify

import (
	"crypto/subtle"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/framework/context"
	"github.com/xxcheng123/cloudpan189-share/internal/pkgs/utils"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

const (
	signerV1Name = "v1"
	v1EncFormat  = "fileId=%d&salt=%s&signer=%s&timestamp=%s&uuid=%s"
)

type SignV1Option struct {
	NoExpire bool
}

type SignV1OptionFunc = func(opt *SignV1Option)

// WithV1NoExpire 设置令牌永不过期
func WithV1NoExpire() SignV1OptionFunc {
	return func(opt *SignV1Option) {
		opt.NoExpire = true
	}
}

func (s *service) SignV1(ctx context.Context, fileId int64, opts ...SignV1OptionFunc) (values url.Values, err error) {
	opt := &SignV1Option{}

	for _, f := range opts {
		f(opt)
	}

	var (
		randUUID  = uuid.NewString()
		timestamp = strconv.FormatInt(time.Now().Add(time.Hour*6).Unix(), 10)
	)

	if opt.NoExpire {
		timestamp = "-1"
	}

	var (
		sign = utils.MD5(fmt.Sprintf(v1EncFormat, fileId, shared.GetSaltKey(), signerV1Name, timestamp, randUUID))
	)

	values = url.Values{
		"signer":    []string{signerV1Name},
		"sign":      []string{sign},
		"timestamp": []string{timestamp},
		"uuid":      []string{randUUID},
	}

	return values, nil
}

// VerifyV1 验证下载文件令牌V1
func (s *service) VerifyV1(ctx context.Context, fileId int64, sign, uuidValue, timestamp, signer string) error {
	// 验证签名器类型
	if signer != signerV1Name {
		return errors.New("无效的签名器类型")
	}

	if _, err := uuid.Parse(uuidValue); err != nil {
		return errors.Wrap(err, "UUID格式无效")
	}

	// 验证时间戳（如果不是永不过期）
	if timestamp != "-1" {
		timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			return errors.Wrap(err, "时间戳格式无效")
		}

		// 检查是否过期
		if time.Now().Unix() > timestampInt {
			return errors.New("令牌已过期")
		}
	}

	// 重新计算签名进行验证
	expectedSign := utils.MD5(fmt.Sprintf(v1EncFormat, fileId, shared.GetSaltKey(), signerV1Name, timestamp, uuidValue))
	if subtle.ConstantTimeCompare([]byte(sign), []byte(expectedSign)) != 1 {
		return errors.New("签名验证失败")
	}

	return nil
}
