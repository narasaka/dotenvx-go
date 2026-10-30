package dotenvx

// Error is an error with one of dotenvx's codes, such as
// "WRONG_PRIVATE_KEY". Its message has dotenvx's "[CODE] message" form,
// followed by the underlying error, if any.
// errors.Is matches an Error against the Err variables by code alone.
type Error struct {
	Code    string
	Message string
	// Err is the underlying error, if any.
	Err error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return "[" + e.Code + "] " + e.Message + ": " + e.Err.Error()
	}
	return "[" + e.Code + "] " + e.Message
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether target is an *Error with the same code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t != nil && t.Code == e.Code
}

// These are for matching with errors.Is. The errors returned are new values
// with the same codes, so changing them doesn't change these. Parse, Read
// and Load report values they couldn't decrypt with an error that matches
// ErrDecryptionFailed and names their keys.
var (
	ErrMissingPrivateKey = &Error{Code: "MISSING_PRIVATE_KEY", Message: "could not decrypt because private key is missing"}
	ErrInvalidPrivateKey = &Error{Code: "INVALID_PRIVATE_KEY", Message: "could not decrypt using private key"}
	ErrWrongPrivateKey   = &Error{Code: "WRONG_PRIVATE_KEY", Message: "could not decrypt using private key"}
	ErrDecryptionFailed  = &Error{Code: "DECRYPTION_FAILED", Message: "could not decrypt value"}
	ErrInvalidPublicKey  = &Error{Code: "INVALID_PUBLIC_KEY", Message: "could not encrypt using public key"}
	ErrEncryptionFailed  = &Error{Code: "ENCRYPTION_FAILED", Message: "could not encrypt value"}
)

// newError returns a new error with the code and message of base.
func newError(base *Error, err error) *Error {
	return &Error{Code: base.Code, Message: base.Message, Err: err}
}
