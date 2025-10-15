package helpers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

var validate = validator.New()

func JSONSuccess(c echo.Context, code int, data any, message string) error {
	if data == nil && message == "" {
		return c.NoContent(http.StatusNoContent)
	}
	if message == "" {
		return c.JSON(code, data)
	}
	if data == nil {
		return c.JSON(code, map[string]string{"message": message})
	}
	return c.JSON(code, map[string]any{"message": message, "data": data})
}

func JSONError(c echo.Context, code int, v any) error {
	var msg string
	switch t := v.(type) {
	case nil:
		msg = http.StatusText(code)
	case string:
		if t == "" {
			msg = http.StatusText(code)
		} else {
			msg = t
		}
	case error:
		if t.Error() == "" {
			msg = http.StatusText(code)
		} else {
			msg = t.Error()
		}
	default:
		msg = fmt.Sprintf("%v", t)
	}
	return c.JSON(code, map[string]string{"error": msg})
}


func BindAndValidate(c echo.Context, v any) error {
	if err := c.Bind(v); err != nil {
		return fmt.Errorf("invalid json")
	}
	if err := validate.Struct(v); err != nil {
		// return a generic human-friendly error (avoid leaking validation internals).
		return fmt.Errorf("missing or invalid fields")
	}
	return nil
}

func BuildShortURL(c echo.Context, baseHost, slug string) string {
	if baseHost != "" {
		return fmt.Sprintf("%s/%s", trimSuffix(baseHost, "/"), slug)
	}
	req := c.Request()
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	host := req.Host
	return fmt.Sprintf("%s://%s/%s", scheme, host, slug)
}

// IsValidURL is a small, permissive URL sanity check used when you need one-off validation.
func IsValidURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	// require scheme and host
	if u.Scheme == "" || u.Host == "" {
		return false
	}
	// simple disallow local schemes like "javascript:" etc.
	sch := strings.ToLower(u.Scheme)
	return sch == "http" || sch == "https"
}

func trimSuffix(s, suf string) string {
	if strings.HasSuffix(s, suf) {
		return s[:len(s)-len(suf)]
	}
	return s
}