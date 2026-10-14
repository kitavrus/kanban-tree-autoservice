package auth

// Wire-типы — JSON только snake_case (§API-1). Идентификатор — login, не адрес почты.

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	// Timezone — часовой пояс пользователя (IANA, DS-031): по нему клиент показывает время.
	Timezone string `json:"timezone"`
}
