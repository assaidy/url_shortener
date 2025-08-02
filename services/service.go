package services

import (
	"fmt"

	"github.com/assaidy/url_shortener/db"
	"github.com/assaidy/url_shortener/repository"
)

type Service interface {
	Start() error
	Stop()
}

var (
	queries = repository.New(db.Connection)

	ConflictErr     = fmt.Errorf("Conflict Error")
	ValidationErr   = fmt.Errorf("Validation Error")
	NotFoundErr     = fmt.Errorf("NotFound Error")
	UnauthorizedErr = fmt.Errorf("Unauthorized Error")
)
