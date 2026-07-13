package main

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrNotFound            = errors.New("ressource introuvable")
	ErrValidation          = errors.New("requête invalide")
	ErrUnauthorized        = errors.New("authentification requise")
	ErrForbidden           = errors.New("action non autorisée")
	ErrConflict            = errors.New("conflit")
	ErrInsufficientCredits = errors.New("crédits insuffisants")
)

type Error struct {
	kind error
	msg  string
}

func (e *Error) Error() string { return e.msg }

func (e *Error) Unwrap() error { return e.kind }

func newError(kind error, format string, args ...any) *Error {
	return &Error{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func validationError(format string, args ...any) *Error {
	return newError(ErrValidation, format, args...)
}

func notFoundError(format string, args ...any) *Error {
	return newError(ErrNotFound, format, args...)
}

func forbiddenError(format string, args ...any) *Error {
	return newError(ErrForbidden, format, args...)
}

func conflictError(format string, args ...any) *Error {
	return newError(ErrConflict, format, args...)
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInsufficientCredits):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
