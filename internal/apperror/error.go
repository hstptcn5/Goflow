package apperror

import (
	"errors"
	"strings"
)

const marker = "goflow-safe-error:"

type Error struct {
	Category string
	Message  string
}

func New(category, message string) error {
	return &Error{Category: strings.TrimSpace(category), Message: strings.TrimSpace(message)}
}

func (e *Error) Error() string {
	return marker + e.Category + ":" + e.Message
}

func Details(err error) (category, message string, ok bool) {
	var publicErr *Error
	if errors.As(err, &publicErr) {
		return publicErr.Category, publicErr.Message, true
	}
	if err == nil {
		return "", "", false
	}
	return DetailsText(err.Error())
}

func DetailsText(text string) (category, message string, ok bool) {
	start := strings.Index(text, marker)
	if start < 0 {
		return "", "", false
	}
	remainder := text[start+len(marker):]
	separator := strings.IndexByte(remainder, ':')
	if separator <= 0 {
		return "", "", false
	}
	category = strings.TrimSpace(remainder[:separator])
	message = strings.TrimSpace(remainder[separator+1:])
	if category == "" || message == "" {
		return "", "", false
	}
	return category, message, true
}
