package advernue

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Error is returned for any non-2xx response. It is built from the
// {"error":{"code","message","details"}} envelope. Use errors.As to read it.
type Error struct {
	Status  int             // HTTP status code
	Code    string          // machine-readable code from the envelope
	Message string          // human-readable message
	Details json.RawMessage // optional structured details
}

func (e *Error) Error() string {
	return fmt.Sprintf("advernue: %d %s: %s", e.Status, e.Code, e.Message)
}

func parseError(status int, body []byte) *Error {
	var env struct {
		Error *struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error != nil {
		return &Error{Status: status, Code: env.Error.Code, Message: env.Error.Message, Details: env.Error.Details}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return &Error{Status: status, Code: fmt.Sprintf("http_%d", status), Message: msg}
}
