package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aidanlsb/perch/internal/config"
	"github.com/aidanlsb/perch/internal/skills"
)

// ErrReported means the failure envelope (or human error) was already written.
var ErrReported = errors.New("cli error reported")

// Response is Raven's CLI envelope: {ok, data, error:{code,message}}.
type Response struct {
	OK    bool       `json:"ok"`
	Data  any        `json:"data,omitempty"`
	Error *ErrorInfo `json:"error,omitempty"`
}

// ErrorInfo is the stable error object inside a failed envelope.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type cliErr struct {
	code    string
	message string
	details any
}

func (e *cliErr) Error() string {
	if e == nil {
		return "cli error"
	}
	if e.code == "" {
		return e.message
	}
	return e.code + ": " + e.message
}

func fail(code, message string) *cliErr {
	return &cliErr{code: code, message: message}
}

func failDetails(code, message string, details any) *cliErr {
	return &cliErr{code: code, message: message, details: details}
}

func writeOK(w io.Writer, jsonOut bool, data any, human string) error {
	if jsonOut {
		return encodeJSON(w, Response{OK: true, Data: data})
	}
	if strings.TrimSpace(human) != "" {
		fmt.Fprintln(w, human)
	}
	return nil
}

func writeErr(stdout, stderr io.Writer, jsonOut bool, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrReported) {
		return err
	}
	ce := asCLI(err)
	if jsonOut {
		if encErr := encodeJSON(stdout, Response{OK: false, Error: &ErrorInfo{
			Code:    ce.code,
			Message: ce.message,
			Details: ce.details,
		}}); encErr != nil {
			fmt.Fprintln(stderr, encErr)
			return errors.Join(ErrReported, encErr)
		}
		return ErrReported
	}
	fmt.Fprintln(stderr, ce.message)
	return ErrReported
}

func asCLI(err error) *cliErr {
	var ce *cliErr
	if errors.As(err, &ce) {
		return ce
	}
	var se *skills.Error
	if errors.As(err, &se) {
		return failDetails(se.Code, se.Message, se.Details)
	}
	var inUse *config.InUseError
	if errors.As(err, &inUse) {
		return failDetails(CodeConfirmationRequired, inUse.Error(), map[string]any{
			"view":    inUse.View,
			"layouts": inUse.Layouts,
		})
	}
	var cfgErr *config.Error
	if errors.As(err, &cfgErr) {
		return failDetails(CodeValidationFailed, cfgErr.Error(), map[string]any{
			"path":    cfgErr.Path,
			"line":    cfgErr.Line,
			"message": cfgErr.Msg,
		})
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "already exists"):
		return fail(CodeDuplicateName, msg)
	case strings.Contains(msg, "not found"):
		return fail(CodeNotFound, msg)
	default:
		return fail(CodeInvalidInput, msg)
	}
}

func encodeJSON(w io.Writer, resp Response) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
