package auth

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type RegisterResponse struct {
	User AuthUser
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	User         AuthUser
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int64
}

type AuthUser struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}
