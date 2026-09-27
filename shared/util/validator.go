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
	case "min", "max":
		bound := "at least"
		if fe.Tag() == "max" {
			bound = "at most"
		}

		// min/max count items on slices (e.g. file uploads) and characters on strings
		unit := "characters"
		if fe.Kind() == reflect.Slice {
			unit = "items"
		}
		return fmt.Sprintf("%s must have %s %s %s", fe.Field(), bound, fe.Param(), unit)
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", fe.Field(), fe.Param())
	case "numeric":
		return fmt.Sprintf("%s must contain only digits", fe.Field())
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
