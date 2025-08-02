package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/assaidy/url_shortener/services"
	"github.com/gofiber/fiber/v2"
)

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleRegister(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json body")
	}

	if err := services.UserServiceInstance.CreateUser(context.Background(), services.CreateUserParams{
		Username: req.Username,
		Password: req.Password,
	}); err != nil {
		return fromServiceError(err)
	}

	return c.SendStatus(fiber.StatusCreated)
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func HandleLogin(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json body")
	}

	jwtToken, err := services.UserServiceInstance.AuthenticateUser(context.Background(), req.Username, req.Password)
	if err != nil {
		return fromServiceError(err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"jwtToken": jwtToken,
	})
}

func WithJwt(c *fiber.Ctx) error {
	tokenString := strings.TrimSpace(strings.TrimPrefix(c.Get(fiber.HeaderAuthorization), "Bearer"))
	if tokenString == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing or malformed Authorization header")
	}

	claims, err := services.UserServiceInstance.ParseJwtTokenString(tokenString)
	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	if time.Until(claims.ExpiresAt.Time) <= 0 {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	if ok, err := services.UserServiceInstance.CheckUsername(context.Background(), claims.Username); err != nil {
		return fromServiceError(err)
	} else if !ok { // user was deleted before token expiration
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	c.Locals(AuthedUsername, claims.Username)
	return c.Next()
}

func HandleDeleteUser(c *fiber.Ctx) error {
	username := c.Locals(AuthedUsername).(string)

	if err := services.UserServiceInstance.DeleteUser(context.Background(), username); err != nil {
		return fromServiceError(err)
	}

	return nil
}
