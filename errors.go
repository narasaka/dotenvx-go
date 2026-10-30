package dotenvx

// Error is an error with one of dotenvx's codes, such as
// "WRONG_PRIVATE_KEY". Its message has dotenvx's "[CODE] message" form.
// errors.Is matches an Error against the Err variables by code alone.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return "[" + e.Code + "] " + e.Message
}

// Is reports whether target is an *Error with the same code.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// The errors that Decrypt, Encrypt and PublicKey return, with dotenvx's
// codes and messages.
var (
	ErrMissingPrivateKey      = &Error{"MISSING_PRIVATE_KEY", "could not decrypt because private key is missing"}
	ErrInvalidPrivateKey      = &Error{"INVALID_PRIVATE_KEY", "could not decrypt using private key"}
	ErrWrongPrivateKey        = &Error{"WRONG_PRIVATE_KEY", "could not decrypt using private key"}
	ErrMalformedEncryptedData = &Error{"MALFORMED_ENCRYPTED_DATA", "could not decrypt because encrypted data appears malformed"}
	ErrInvalidPublicKey       = &Error{"INVALID_PUBLIC_KEY", "could not encrypt using public key"}
)
