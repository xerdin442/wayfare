package util

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
)

func formatErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", fe.Field())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", fe.Field())
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", fe.Field(), fe.Param())
	default:
		return fmt.Sprintf("%s is invalid", fe.Field())
	}
}

func FormatValidationErrors(err error, req any) map[string]string {
	fieldErrors := make(map[string]string)

	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return fieldErrors
	}

	t := reflect.TypeOf(req)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for _, fe := range ve {
		formName := fe.Field()
		if field, ok := t.FieldByName(fe.Field()); ok {
			tagName := field.Tag.Get("form")
			if tagName == "" {
				tagName = field.Tag.Get("json")
			}
			if tagName != "" {
				formName = tagName
			}
		}
		fieldErrors[formName] = formatErrorMessage(fe)
	}

	return fieldErrors
}
