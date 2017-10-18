package none

// TokenResponse implementation for None authentication
type TokenResponse struct {
}

// Method is set to "None"
func (tr TokenResponse) Method() string {
	return "None"
}

// Token is set to a dummy string
func (tr TokenResponse) Token() string {
	return ""
}

// ExpiresIn is set to 0 for None authentication
func (tr TokenResponse) ExpiresIn() int {
	return 0
}

// AuthHeader returns contents of the Authorization HTTP header
func (tr TokenResponse) AuthHeader() string {
	return tr.Method() + " " + tr.Token()
}

// RequestToken does pretty little here...
func RequestToken() (*TokenResponse, error) {
	return &TokenResponse{}, nil
}
