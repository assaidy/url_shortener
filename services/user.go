package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/assaidy/url_shortener/config"
	"github.com/assaidy/url_shortener/repository"
	"github.com/assaidy/url_shortener/utils"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var UserServiceInstance *UserService

type UserService struct{}

func (me *UserService) Start() error {
	slog.Info("user service started")
	return nil
}

func (me *UserService) Stop() {
	slog.Info("user service stopped")
}

type CreateUserParams struct {
	Username string `validate:"required,customUsername,max=20"`
	Password string `validate:"required,customNoOuterSpaces,min=8,max=50"`
}

func (me *UserService) CreateUser(ctx context.Context, params CreateUserParams) error {
	if err := utils.ValidateStruct(params); err != nil {
		return fmt.Errorf("%w: %s", ValidationErr, err.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error hashing password: %w", err)
	}

	numAffectedRows, err := queries.InsertUser(ctx, repository.InsertUserParams{
		Username:       params.Username,
		HashedPassword: string(hashedPassword),
	})
	if numAffectedRows == 0 {
		return fmt.Errorf("%w: %s", ConflictErr, "username already exists")
	}

	return nil
}

// checks username and password, and returns a jwt token if authenticated
func (me *UserService) AuthenticateUser(ctx context.Context, username string, password string) (string, error) {
	user, err := queries.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("%w: %s", UnauthorizedErr, "invalid username")
		}
		return "", fmt.Errorf("error getting user from db: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password)); err != nil {
		return "", fmt.Errorf("%w: %s", UnauthorizedErr, "invalid password")
	}

	token, err := generateJWTAccessToken(JwtClaims{
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.JwtTokenExpirationDays) * time.Minute)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("error generating jwt token: %w", err)
	}

	return token, nil
}

type JwtClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func generateJWTAccessToken(claims JwtClaims) (string, error) {
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return jwtToken.SignedString([]byte(config.SecretKey))
}

func (me *UserService) ParseJwtTokenString(tokenString string) (*JwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JwtClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(config.SecretKey), nil
	})
	if err != nil {
		return nil, jwt.ErrTokenSignatureInvalid
	}

	claims, ok := token.Claims.(*JwtClaims)
	if !ok {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (me *UserService) DeleteUser(ctx context.Context, username string) error {
	if numAffectedRows, err := queries.DeleteUserByUsername(ctx, username); err != nil {
		return fmt.Errorf("error checking username: %w", err)
	} else if numAffectedRows == 0 {
		return fmt.Errorf("%w: user not found", NotFoundErr)
	}

	return nil
}
