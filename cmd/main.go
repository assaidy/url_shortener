package main

import (
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/assaidy/url_shortener/config"
	"github.com/assaidy/url_shortener/handlers"
	"github.com/assaidy/url_shortener/services"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
	// NOTE: Logging occurs before this error handler is executed, so the internal error
	// has already been logged. We avoid exposing internal error details to the client.
	if code == fiber.StatusInternalServerError {
		return c.SendStatus(code)
	}
	return c.Status(code).SendString(err.Error())
}

func registerRoutes(router *fiber.App) {
	router.Use(logger.New())

	router.Post("/users/register", handlers.HandleRegister)
	router.Post("/users/login", handlers.HandleLogin)
	router.Delete("/users", handlers.WithJwt, handlers.HandleDeleteUser)

	router.Post("/urls", handlers.WithJwt, handlers.HandleCreateShortUrl)
}

func main() {
	services := []services.Service{
		services.UserServiceInstance,
		services.UrlServiceInstance,
	}

	for index, it := range services {
		if err := it.Start(); err != nil {
			slog.Error("error starting service", "index", index, "err", err)
			os.Exit(1)
		}
	}

	app := fiber.New(fiber.Config{
		AppName:      "URL Shortener",
		ServerHeader: "URL Shortener",
		ErrorHandler: errorHandler,
		Prefork:      true,
	})

	registerRoutes(app)

	go func() {
		if err := app.Listen(config.ServerAddr); err != nil {
			slog.Error("error starting server", "err", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("starting server shutdown...")

	for _, it := range services {
		it.Stop()
	}

	if err := app.ShutdownWithTimeout(2 * time.Second); err != nil {
		slog.Error("error shutdown server", "err", err)
	} else {
		slog.Info("server shutdown completed successfully", "pid", os.Getpid())
	}
}
