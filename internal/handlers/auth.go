package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// struct used for unmarshalling the register request
type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// struct for marhsalling the register response
type registerResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// struct for unmarshalling the register request
type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// struct for marshalling the register response
type loginResponse struct {
	Token    string `json:"token"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Register handler
func Register(conn *sql.DB) http.HandlerFunc {
	// the closure function takes a response writer and a pointer to a http request
	return func(w http.ResponseWriter, r *http.Request) {
		// defining a data structure for the request so it can be unmarshalled
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// hashing the password
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "could not process password", http.StatusInternalServerError)
			return
		}

		// creating a new UUID to insert into the database
		id := uuid.New().String()

		_, err = conn.Exec(
			`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4)`,
			id, req.Username, req.Email, string(hash),
		)
		if err != nil {
			var pgErr *pgconn.PgError
			// Throwing an error if the username or email was already taken
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(`{"error": {"code": "USERNAME_TAKEN", "message": "that username or email is already registered"}}`))
				return
			}
			http.Error(w, "could not create user", http.StatusInternalServerError)
			return
		}

		// Creating a response to the user creation
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(registerResponse{ID: id, Username: req.Username})

	}

}

func Login(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var id, username, passwordHash string
		err := conn.QueryRow(
			`SELECT id, username, password_hash FROM users WHERE username = $1 OR email = $1`,
			req.Identifier,
		).Scan(&id, &username, &passwordHash)

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"code": "INVALID_CREDENTIALS", "message": "incorrect username/email or password"}}`))
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": {"code": "INVALID_CREDENTIALS", "message": "incorrect username/email or password"}}`))
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": id,
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})

		signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			http.Error(w, "could not generate token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(loginResponse{Token: signedToken, ID: id, Username: username})
	}
}
