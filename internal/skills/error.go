package skills

// Stable skill error codes. Names match Raven's envelope contract.
const (
	CodeNotFound       = "SKILL_NOT_FOUND"
	CodeNotInstalled   = "SKILL_NOT_INSTALLED"
	CodePathUnresolved = "SKILL_PATH_UNRESOLVED"
	CodeInvalidInput   = "INVALID_INPUT"
	CodeFileWrite      = "FILE_WRITE_ERROR"
)

// Error is a coded skill-command failure.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string {
	if e == nil {
		return "skill error"
	}
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

func fail(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func failDetails(code, message string, details any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}
