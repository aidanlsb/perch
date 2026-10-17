package cli

// Stable error codes. Names match Raven's envelope contract where they apply.
const (
	CodeInvalidArgs          = "INVALID_ARGS"
	CodeMissingArgument      = "MISSING_ARGUMENT"
	CodeNotFound             = "NOT_FOUND"
	CodeDuplicateName        = "DUPLICATE_NAME"
	CodeValidationFailed     = "VALIDATION_FAILED"
	CodeConfigInvalid        = "CONFIG_INVALID"
	CodeFileReadError        = "FILE_READ_ERROR"
	CodeFileWriteError       = "FILE_WRITE_ERROR"
	CodeConfirmationRequired = "CONFIRMATION_REQUIRED"
	CodeCommandNotFound      = "COMMAND_NOT_FOUND"
	CodeInvalidInput         = "INVALID_INPUT"
	CodeInvalidJSON          = "INVALID_JSON"
	CodeMultipleMatches      = "MULTIPLE_MATCHES"
)

// ErrorCodes is the documented set agents can branch on.
var ErrorCodes = []string{
	CodeInvalidArgs,
	CodeMissingArgument,
	CodeNotFound,
	CodeDuplicateName,
	CodeValidationFailed,
	CodeConfigInvalid,
	CodeFileReadError,
	CodeFileWriteError,
	CodeConfirmationRequired,
	CodeCommandNotFound,
	CodeInvalidInput,
	CodeInvalidJSON,
	CodeMultipleMatches,
}
