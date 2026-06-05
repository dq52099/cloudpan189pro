package user

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/xxcheng123/cloudpan189-share/internal/shared"
)

// Claims JWT令牌结构体
type Claims struct {
	UserId      int64  `json:"user_id"`
	Username    string `json:"username"`
	UserVersion int    `json:"user_version"`
	jwt.RegisteredClaims
}

// Token过期时间
const (
	AccessTokenExpire  = time.Hour * 24     // 24小时
	RefreshTokenExpire = time.Hour * 24 * 7 // 7天
)

// ParseAccessToken 解析访问Token
func (s *service) ParseAccessToken(tokenString string) (int64, string, int, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(shared.GetSaltKey()), nil
	})
	if err != nil {
		return 0, "", 0, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.Subject != "access-token" {
			return 0, "", 0, errors.New("invalid access token subject")
		}

		return claims.UserId, claims.Username, claims.UserVersion, nil
	}

	return 0, "", 0, errors.New("invalid access token")
}

// GenerateAccessToken 生成访问Token
func (s *service) GenerateAccessToken(userId int64, username string, userVersion int) (string, error) {
	claims := &Claims{
		UserId:      userId,
		Username:    username,
		UserVersion: userVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Subject:   "access-token",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(shared.GetSaltKey()))
}

// GenerateRefreshToken 生成刷新Token
func (s *service) GenerateRefreshToken(userId int64, username string, userVersion int) (string, error) {
	claims := &Claims{
		UserId:      userId,
		Username:    username,
		UserVersion: userVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Subject:   "refresh-token",
			ID:        fmt.Sprintf("%d", userId),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(shared.GetSaltKey()))
}

// ParseRefreshToken 解析刷新Token
func (s *service) ParseRefreshToken(tokenString string) (int64, string, int, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(shared.GetSaltKey()), nil
	})
	if err != nil {
		return 0, "", 0, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		if claims.Subject != "refresh-token" {
			return 0, "", 0, errors.New("invalid refresh token subject")
		}

		return claims.UserId, claims.Username, claims.UserVersion, nil
	}

	return 0, "", 0, errors.New("invalid refresh token")
}
