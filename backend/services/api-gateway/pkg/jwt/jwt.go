package jwt

import (
	"api-gateway/pkg/exception"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// MySecret 定义JWT TOKEN的加密盐
var MySecret = []byte("gateway")
var ExpiresTime = time.Now().Add(2 * time.Hour)

type MyClaims struct {
	Username   string `json:"username"`
	UserId     int64  `json:"userId"`
	SuperAdmin bool   `json:"superAdmin"`
	RoleId     int64  `json:"roleId"`
	DataScope  string `json:"dataScope"`
	jwt.RegisteredClaims
}

func CreateToken(username string, userId int64) (string, string, time.Time, error) {
	return CreateTokenWithClaims(username, userId, 0, false)
}

func CreateTokenWithClaims(username string, userId, roleId int64, superAdmin bool) (string, string, time.Time, error) {
	expiresTime := time.Now().Add(2 * time.Hour)
	claims := &MyClaims{
		Username:         username,
		UserId:           userId,
		RoleId:           roleId,
		SuperAdmin:       superAdmin,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresTime)},
	}
	accessTokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := accessTokenClaims.SignedString(MySecret)
	if err != nil {
		return "", "", expiresTime, err
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256,
		&MyClaims{
			Username:         username,
			UserId:           userId,
			RoleId:           roleId,
			SuperAdmin:       superAdmin,
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresTime.Add(24 * time.Hour))},
		}).SignedString(MySecret)
	return accessToken, refreshToken, expiresTime, err
}

func ParseToken(tokenString string) (*MyClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MyClaims{}, func(token *jwt.Token) (interface{}, error) {
		return MySecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*MyClaims); ok && token.Valid { // 校验token
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// RefreshToken 刷新AccessToken
func RefreshToken(aToken, rToken string) (newAToken, newRToken string, expiresAt time.Time, err error) {
	var refreshClaims MyClaims
	refreshToken, err := jwt.ParseWithClaims(rToken, &refreshClaims, func(token *jwt.Token) (interface{}, error) {
		return MySecret, nil
	})
	if err != nil || refreshToken == nil || !refreshToken.Valid {
		err = exception.NewRefreshTokenExpired("需要登陆")
		return
	}

	var claims MyClaims
	_, err = jwt.ParseWithClaims(aToken, &claims, func(token *jwt.Token) (interface{}, error) {
		return MySecret, nil
	})
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return
	}
	if claims.Username == "" || claims.UserId == 0 {
		err = exception.NewAccessTokenIllegal("Authorization error, invalid access token claims")
		return
	}
	return CreateTokenWithClaims(claims.Username, claims.UserId, claims.RoleId, claims.SuperAdmin)
}
