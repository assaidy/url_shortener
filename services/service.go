package services

import (
	"fmt"
)

type Service interface {
	Start() error
	Stop()
}

var (
	ConflictErr     = fmt.Errorf("Conflict Error")
	ValidationErr   = fmt.Errorf("Validation Error")
	NotFoundErr     = fmt.Errorf("NotFound Error")
	UnauthorizedErr = fmt.Errorf("Unauthorized Error")
)
