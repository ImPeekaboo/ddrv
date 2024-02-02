package api

import (
	"github.com/gofiber/fiber/v2"

	dp "github.com/forscht/ddrv/internal/dataprovider"
)

const (
	StatusOk                  = fiber.StatusOK
	StatusRangeNotSatisfiable = fiber.StatusRequestedRangeNotSatisfiable
	StatusPartialContent      = fiber.StatusPartialContent
	StatusBadRequest          = fiber.StatusBadRequest
	StatusNotFound            = fiber.StatusNotFound
	StatusForbidden           = fiber.StatusForbidden
	StatusUnauthorized        = fiber.StatusUnauthorized
	StatusCreated             = fiber.StatusCreated
)

const (
	ErrBadRequest          = "bad request body"
	ErrUnauthorized        = "authorization failed"
	ErrBadUsernamePassword = "invalid username or password"
)

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Directory struct {
	*dp.File
	Files []*dp.File `json:"files"`
}
