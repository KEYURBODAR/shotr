package helpers

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate = validator.New()

// JSONSuccess writes a concise success response.
// Behavior:
// - if data == nil && message == "" -> 204 No Content
// - if data != nil && message == "" -> write data directly with given code
// - if data == nil && message != "" -> {"message": message}
// - if data != nil && message != "" -> {"message": message, "data": data}
func JSONSuccess(c echo.Context, code int, data any, message string) error {
	// 204 when nothing to return
	if data == nil && message == "" {
		return c.NoContent(http.StatusNoContent)
	}

	// If message absent, return payload directly (clean, typed)
	if message == "" {
		return c.JSON(code, data)
	}

	// Message present: wrap message and include data only if provided
	if data == nil {
		return c.JSON(code, map[string]string{"message": message})
	}
	return c.JSON(code, map[string]any{"message": message, "data": data})
}

// JSONError writes {"error":"<text>"} with the provided HTTP code.
// Accepts string, error, or any type (it will be fmt.Sprintf'd).
func JSONError(c echo.Context, code int, err any) error {
	var msg string
	switch v := err.(type) {
	case nil:
		msg = http.StatusText(code)
	case string:
		if v == "" {
			msg = http.StatusText(code)
		} else {
			msg = v
		}
	case error:
		if v.Error() == "" {
			msg = http.StatusText(code)
		} else {
			msg = v.Error()
		}
	default:
		msg = fmt.Sprintf("%v", v)
	}
	return c.JSON(code, map[string]string{"error": msg})
}

func BindAndValidate(c echo.Context, req any) error {
	if err := c.Bind(req); err != nil {
		return JSONError(c, http.StatusBadRequest, "invalid json")
	}

	if err := validate.Struct(req); err != nil {
		return JSONError(c, http.StatusBadRequest, "missing or invalid fields")
	}
	
	return nil
}