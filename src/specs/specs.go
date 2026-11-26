// Package specs documents the HTTP API.
package specs

import (
	"fmt"
	"strings"

	"github.com/assaidy/moon"
)

var (
	sessionSecurity = moon.ApiSecurityRequirement{
		"sessionId":    {},
		"sessionToken": {},
	}

	sessionCsrfSecurity = moon.ApiSecurityRequirement{
		"sessionId":    {},
		"sessionToken": {},
		"csrfToken":    {},
	}
)

// documentedErrorStatuses maps every documented error kind to the status code
// it is returned with. A kind is the identifier clients use to distinguish
// errors, so one kind must never be documented under more than one status.
var documentedErrorStatuses = map[string]int{}

// emptyResponse builds a response that carries no body.
func emptyResponse(status int, description string) moon.ApiResponse {
	return moon.ApiResponse{
		Status:      status,
		Description: description,
	}
}

// jsonResponse builds a response with a JSON body.
func jsonResponse(status int, description string, schema any) moon.ApiResponse {
	return moon.ApiResponse{
		Status:      status,
		Description: description,
		Content: map[string]moon.ApiMediaType{
			"application/json": {Schema: schema},
		},
	}
}

// errorResponse documents the error kinds that can be returned under one status
// code. OpenAPI allows only one response per status, so kinds sharing a status
// are grouped into a single response with an enum and one example per kind.
//
// Error kinds are the client-facing identifiers: a status may map to several
// kinds, but a kind must always map to the same status.
func errorResponse(status int, description string, errs ...moon.Error) moon.ApiResponse {
	moon.Assert(len(errs) > 0, "errorResponse requires at least one error kind")

	response := jsonResponse(status, description, moon.Object{
		"$ref": "#/components/schemas/Error",
	})

	names := make([]string, len(errs))
	examples := make(map[string]any, len(errs))
	for i, err := range errs {
		moon.Assert(err.StatusCode == status, fmt.Sprintf(
			"error kind %q has status %d but is documented under status %d",
			err.Kind, err.StatusCode, status,
		))

		if previous, ok := documentedErrorStatuses[err.Kind]; ok {
			moon.Assert(previous == status, fmt.Sprintf(
				"error kind %q is documented with statuses %d and %d",
				err.Kind, previous, status,
			))
		}
		documentedErrorStatuses[err.Kind] = status

		names[i] = err.Kind
		examples[err.Kind] = moon.Object{
			"summary": err.Kind,
			"value": moon.Object{
				"kind":        err.Kind,
				"description": err.Description,
			},
		}
	}

	response.Content = map[string]moon.ApiMediaType{
		"application/json": {
			Schema: moon.Object{
				"allOf": []any{
					moon.Object{"$ref": "#/components/schemas/Error"},
					moon.Object{
						"type": "object",
						"properties": moon.Object{
							"kind": moon.Object{
								"type": "string",
								"enum": names,
							},
						},
					},
				},
			},
			Examples: examples,
		},
	}
	return response
}

// jsonRequestBody builds a required JSON request body.
func jsonRequestBody(description string, schema any) moon.ApiRequestBody {
	return moon.ApiRequestBody{
		Description: description,
		Required:    true,
		Content: map[string]moon.ApiMediaType{
			"application/json": {Schema: schema},
		},
	}
}

// uuidPathParameter documents a UUID path parameter.
func uuidPathParameter(name, description string) moon.ApiParameter {
	return moon.ApiParameter{
		Name:        name,
		In:          moon.ApiParamInPath,
		Description: description,
		Required:    true,
		Schema:      moon.Object{"type": "string", "format": "uuid"},
	}
}

// aliasPathParameter documents the public link alias path parameter.
func aliasPathParameter() moon.ApiParameter {
	return moon.ApiParameter{
		Name:        "alias",
		In:          moon.ApiParamInPath,
		Description: "The public alias of the short link.",
		Required:    true,
		Schema:      moon.Object{"type": "string"},
	}
}

// cursorParameter documents the cursor query parameter used by cursor-based
// pagination.
func cursorParameter() moon.ApiParameter {
	return moon.ApiParameter{
		Name:        "cursor",
		In:          moon.ApiParamInQuery,
		Description: "The opaque cursor returned as nextCursor by the previous page.",
		Schema:      moon.Object{"type": "string"},
	}
}

// limitParameter documents the limit query parameter used by pagination.
func limitParameter() moon.ApiParameter {
	return moon.ApiParameter{
		Name:        "limit",
		In:          moon.ApiParamInQuery,
		Description: "The maximum number of items to return.",
		Schema: moon.Object{
			"type":    "integer",
			"minimum": 1,
			"maximum": 100,
			"default": 10,
		},
	}
}

// searchParameter documents the search query parameter used to filter lists.
func searchParameter(description string) moon.ApiParameter {
	return moon.ApiParameter{
		Name:        "search",
		In:          moon.ApiParamInQuery,
		Description: description,
		Schema:      moon.Object{"type": "string"},
	}
}

// sortParameter documents the sort query parameter used by pagination.
func sortParameter(keys ...string) moon.ApiParameter {
	return moon.ApiParameter{
		Name:        "sort",
		In:          moon.ApiParamInQuery,
		Description: "The sort key, optionally prefixed with - for descending order. Only one key is allowed. Allowed keys: " + strings.Join(keys, ", ") + ".",
		Schema:      moon.Object{"type": "string"},
	}
}

// pageSchema builds a cursor-paginated response schema around itemSchema.
func pageSchema(itemSchema any) moon.Object {
	return moon.Object{
		"type":     "object",
		"required": []string{"items", "nextCursor"},
		"properties": moon.Object{
			"items": moon.Object{
				"type":  "array",
				"items": itemSchema,
			},
			"nextCursor": moon.Object{
				"type":        "string",
				"description": "The cursor for the next page, or an empty string when there are no more items.",
			},
		},
	}
}
