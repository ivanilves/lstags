package none

// Token implementation for None authentication
type Token struct {
}

// Method is set to "None"
func (tk Token) Method() string {
	return "None"
}

// String is empty (no token here)
func (tk Token) String() string {
	return ""
}

// ExpiresIn is set to 0 for None authentication
func (tk Token) ExpiresIn() int {
	return 0
}

// RequestToken does pretty little here...
func RequestToken() (*Token, error) {
	return &Token{}, nil
}
