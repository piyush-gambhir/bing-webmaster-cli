package client

import (
	"fmt"
	"strings"
	"unicode"
)

// ErrorNames is Bing's ApiErrorCode enum.
var ErrorNames = map[int]string{
	0: "None", 1: "InternalError", 2: "UnknownError", 3: "InvalidApiKey", 4: "ThrottleUser",
	5: "ThrottleHost", 6: "UserBlocked", 7: "InvalidUrl", 8: "InvalidParameter", 9: "TooManySites",
	10: "UserNotFound", 11: "NotFound", 12: "AlreadyExists", 13: "NotAllowed", 14: "NotAuthorized",
	15: "UnexpectedState", 16: "Deprecated",
}

var hints = map[int]string{
	3:  "the API key is invalid; a newly generated key can take about 30 minutes to start working (check BWT_API_KEY or run bwt auth login --with-api-key)",
	4:  "Bing is throttling this user; wait before trying again (no automatic retry was attempted)",
	5:  "Bing is throttling this host; wait before trying again (no automatic retry was attempted)",
	6:  "Bing reports this user as blocked",
	7:  "Bing rejected a URL; site URLs must match bwt sites list exactly, including scheme and trailing slash",
	8:  "Bing rejected a parameter value",
	9:  "the account has reached Bing's site limit",
	11: "Bing found no matching item",
	12: "the item already exists",
	13: "Bing does not allow this operation for this site or account",
	14: "not authorized for this site; check that the site URL matches bwt sites list exactly and that you have access",
	16: "Bing reports this method as deprecated",
}

// APIError is a failed Bing call. Code is -1 when Bing returned no fault body.
type APIError struct {
	Method     string `json:"method" yaml:"method"`
	HTTPStatus int    `json:"http_status" yaml:"http_status"`
	Code       int    `json:"api_error_code" yaml:"api_error_code"`
	Name       string `json:"api_error_name,omitempty" yaml:"api_error_name,omitempty"`
	Message    string `json:"message" yaml:"message"`
	RetryAfter string `json:"retry_after,omitempty" yaml:"retry_after,omitempty"`
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("Bing %s failed (HTTP %d)", e.Method, e.HTTPStatus)
	if e.Name != "" {
		msg = fmt.Sprintf("Bing %s failed: %s (code %d, HTTP %d)", e.Method, e.Name, e.Code, e.HTTPStatus)
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.RetryAfter != "" {
		msg += "; Retry-After: " + e.RetryAfter
	}
	return msg
}

func newFault(method string, status, code int, serverMessage string) *APIError {
	e := &APIError{Method: method, HTTPStatus: status, Code: code, Name: ErrorNames[code]}
	if e.Name == "" {
		e.Name = fmt.Sprintf("Unknown(%d)", code)
	}
	msg := hints[code]
	if serverMessage != "" && serverMessage != e.Name {
		if msg != "" {
			msg += "; "
		}
		msg += "Bing says: " + serverMessage
	}
	e.Message = msg
	return e
}

// clean makes server text safe for a terminal: control characters become spaces
// and long text is cut.
func clean(s string, limit int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		s = s[:limit] + "..."
	}
	return s
}
