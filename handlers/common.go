package handlers

import (
	"github.com/assaidy/url_shortener/services"
	"github.com/gofiber/fiber/v2"
)

const (
	AuthedUsername = "middleware.jwt.AuthedUsername"
)

func fromServiceError(serviceErr error) error {
	if serviceErr == nil {
		panic("function should not be called on nil errors")
	}

	status := fiber.StatusInternalServerError

	switch serviceErr {
	case services.ConflictErr:
		status = fiber.StatusConflict
	case services.NotFoundErr:
		status = fiber.StatusNotFound
	case services.UnauthorizedErr:
		status = fiber.StatusUnauthorized
	case services.ValidationErr:
		status = fiber.StatusBadRequest
	}

	return fiber.NewError(status, serviceErr.Error())
}
