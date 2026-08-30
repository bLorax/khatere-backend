// Package httpapi holds HTTP adapters. Each handler in this package does
// three jobs only: decode the request, call one use case, encode the
// response. A handler holds no business rules and no SQL.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	appuser "yadegar/internal/application/user"
	domainuser "yadegar/internal/domain/user"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type loginResponse struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

// UserHandler wires HTTP routes to user use cases.
type UserHandler struct {
	register *appuser.RegisterUseCase
	login    *appuser.LoginUseCase
}

func NewUserHandler(register *appuser.RegisterUseCase, login *appuser.LoginUseCase) *UserHandler {
	return &UserHandler{register: register, login: login}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	u, err := h.register.Execute(r.Context(), appuser.RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, domainuser.ErrUsernameTaken) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"error": {"code": "USERNAME_TAKEN", "message": "that username or email is already registered"}}`))
			return
		}
		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registerResponse{ID: u.ID, Username: u.Username})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	out, err := h.login.Execute(r.Context(), appuser.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"code": "INVALID_CREDENTIALS", "message": "incorrect username/email or password"}}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loginResponse{Token: out.Token, ID: out.ID, Username: out.Username})
}
