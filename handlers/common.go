package handlers

import (
	"errors"

	"github.com/assaidy/url_shortener/services"
	"github.com/gofiber/fiber/v2"
)

const (
	AuthedUsername = "middleware.jwt.AuthedUsername"
)

func fromServiceError(err error) error {
	if err == nil {
		panic("function should not be called on nil errors")
	}

	is := func(serviceErr error) bool {
		return errors.Is(err, serviceErr) 
	}

	status := fiber.StatusInternalServerError
	switch {
	case is(services.ConflictErr):     status = fiber.StatusConflict
	case is(services.NotFoundErr):     status = fiber.StatusNotFound
	case is(services.UnauthorizedErr): status = fiber.StatusUnauthorized
	case is(services.ValidationErr):   status = fiber.StatusBadRequest
	}

	return fiber.NewError(status, err.Error())
}
