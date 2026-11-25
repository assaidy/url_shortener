package specs

import "github.com/assaidy/moon"

// Components holds the reusable OpenAPI components served by the Swagger UI.
var Components = map[string]any{
	"securitySchemes": map[string]any{
		"sessionId": map[string]any{
			"type": "apiKey",
			"in":   "cookie",
			"name": "session_id",
		},
		"sessionToken": map[string]any{
			"type": "apiKey",
			"in":   "cookie",
			"name": "session_token",
		},
		"csrfToken": map[string]any{
			"type": "apiKey",
			"in":   "cookie",
			"name": "csrf_token",
		},
	},
	"schemas": map[string]any{
		"Error":           errorSchema,
		"Account":         accountSchema,
		"Url":             urlSchema,
		"Redirection":     redirectionSchema,
		"UrlPage":         urlPageSchema,
		"RedirectionPage": redirectionPageSchema,
	},
}

var (
	errorSchema = moon.Object{
		"type":     "object",
		"required": []string{"kind", "description"},
		"properties": moon.Object{
			"kind":        moon.Object{"type": "string"},
			"description": moon.Object{"type": "string"},
			"details":     moon.Object{},
		},
	}

	accountSchema = moon.Object{
		"type":     "object",
		"required": []string{"id", "name", "username", "createdAt"},
		"properties": moon.Object{
			"id":        moon.Object{"type": "string", "format": "uuid"},
			"name":      moon.Object{"type": "string"},
			"username":  moon.Object{"type": "string"},
			"createdAt": moon.Object{"type": "string", "format": "date-time"},
		},
	}

	urlSchema = moon.Object{
		"type":     "object",
		"required": []string{"id", "userId", "title", "url", "alias", "createdAt", "expiresAt", "isActive", "clicks"},
		"properties": moon.Object{
			"id":        moon.Object{"type": "string", "format": "uuid"},
			"userId":    moon.Object{"type": "string", "format": "uuid"},
			"title":     moon.Object{"type": "string", "maxLength": 256},
			"url":       moon.Object{"type": "string", "format": "uri"},
			"alias":     moon.Object{"type": "string"},
			"createdAt": moon.Object{"type": "string", "format": "date-time"},
			"expiresAt": moon.Object{"type": []string{"string", "null"}, "format": "date-time"},
			"isActive":  moon.Object{"type": "boolean"},
			"clicks":    moon.Object{"type": "integer", "format": "int64", "minimum": 0},
		},
	}

	redirectionSchema = moon.Object{
		"type":     "object",
		"required": []string{"id", "urlId", "userId", "ip", "platform", "os", "createdAt"},
		"properties": moon.Object{
			"id":        moon.Object{"type": "string", "format": "uuid"},
			"urlId":     moon.Object{"type": "string", "format": "uuid"},
			"userId":    moon.Object{"type": "string", "format": "uuid"},
			"ip":        moon.Object{"type": "string"},
			"platform":  moon.Object{"type": "string"},
			"os":        moon.Object{"type": "string"},
			"createdAt": moon.Object{"type": "string", "format": "date-time"},
		},
	}

	urlPageSchema         = pageSchema(urlSchema)
	redirectionPageSchema = pageSchema(redirectionSchema)

	createUrlRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"url", "title"},
		"properties": moon.Object{
			"url":       moon.Object{"type": "string", "format": "uri"},
			"title":     moon.Object{"type": "string", "maxLength": 256},
			"expiresAt": moon.Object{"type": []string{"string", "null"}, "format": "date-time"},
		},
	}

	editUrlRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"title"},
		"properties": moon.Object{
			"title":     moon.Object{"type": "string", "maxLength": 256},
			"expiresAt": moon.Object{"type": []string{"string", "null"}, "format": "date-time"},
			"isActive":  moon.Object{"type": "boolean"},
		},
	}

	registerRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"username", "name", "password", "verifyPassword"},
		"properties": moon.Object{
			"username": moon.Object{
				"type":      "string",
				"minLength": 1,
				"maxLength": 32,
				"pattern":   `^[a-zA-Z0-9_]+$`,
			},
			"name": moon.Object{
				"type":      "string",
				"minLength": 1,
				"maxLength": 256,
			},
			"password": moon.Object{
				"type":      "string",
				"minLength": 8,
				"maxLength": 50,
			},
			"verifyPassword": moon.Object{
				"type":      "string",
				"minLength": 8,
				"maxLength": 50,
			},
		},
	}

	loginRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"username", "password"},
		"properties": moon.Object{
			"username": moon.Object{"type": "string", "minLength": 1},
			"password": moon.Object{"type": "string", "minLength": 1},
		},
	}

	editUsernameRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"username"},
		"properties": moon.Object{
			"username": moon.Object{
				"type":      "string",
				"minLength": 1,
				"maxLength": 32,
				"pattern":   `^[a-zA-Z0-9_]+$`,
			},
		},
	}

	editPasswordRequestSchema = moon.Object{
		"type":     "object",
		"required": []string{"oldPassword", "newPassword", "verifyNewPassword"},
		"properties": moon.Object{
			"oldPassword": moon.Object{"type": "string", "minLength": 1},
			"newPassword": moon.Object{
				"type":      "string",
				"minLength": 8,
				"maxLength": 50,
			},
			"verifyNewPassword": moon.Object{
				"type":      "string",
				"minLength": 8,
				"maxLength": 50,
			},
		},
	}
)
