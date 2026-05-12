package auth

type RegisterRequest struct {
	Email       string
	Password    string
	DisplayName string
}

type RegisterResponse struct {
	User AuthUser
}

type LoginRequest struct {
	Email    string
	Password string
}

type LoginResponse struct {
	User         AuthUser
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

type RefreshRequest struct {
	RefreshToken string
}

type RefreshResponse struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
	User         AuthUser
}

type LogoutRequest struct {
	RefreshToken string
}

type MeResponse struct {
	User AuthUser
}

type AuthUser struct {
	ID          int64
	Email       string
	DisplayName string
}
