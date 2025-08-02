package handlers

import (
	"context"

	"github.com/assaidy/url_shortener/services"
	"github.com/gofiber/fiber/v2"
)

type CreateShortUrlRequest struct {
	LongUrl  string `json:"longUrl"`
	ShortUrl string `json:"shortUrl"`
}

func HandleCreateShortUrl(c *fiber.Ctx) error {
	var req CreateShortUrlRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json body")
	}

	username := c.Locals(AuthedUsername).(string)

	shortUrl, err := services.UrlServiceInstance.CreateShortUrl(context.Background(), services.CreateShortUrlParams{
		Username: username,
		LongUrl:  req.LongUrl,
		ShortUrl: req.ShortUrl,
	})
	if err != nil {
		return fromServiceError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"shortUrl": shortUrl,
	})
}
