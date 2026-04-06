package apperror

import "net/http"

type AppError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

func NotFound(resource, detail string) *AppError {
	return &AppError{Code: http.StatusNotFound, Message: resource + " not found", Detail: detail}
}

func Validation(detail string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: "validation error", Detail: detail}
}

func Unauthorized(detail string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, Message: "unauthorized", Detail: detail}
}

func Forbidden(detail string) *AppError {
	return &AppError{Code: http.StatusForbidden, Message: "forbidden", Detail: detail}
}

func Conflict(detail string) *AppError {
	return &AppError{Code: http.StatusConflict, Message: "conflict", Detail: detail}
}

func Internal(detail string) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: "internal error", Detail: detail}
}
