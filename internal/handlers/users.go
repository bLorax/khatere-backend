package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

type userSearchResult struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type userResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func SearchUsers(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")

		if len(q) < 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"results": []userSearchResult{}})
			return
		}

		rows, err := conn.Query(
			`SELECT id, username FROM users WHERE username ILIKE '%' || $1 || '%' ORDER BY username LIMIT 20`,
			q,
		)
		if err != nil {
			http.Error(w, "search failed", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		results := []userSearchResult{}
		for rows.Next() {
			var u userSearchResult
			if err := rows.Scan(&u.ID, &u.Username); err != nil {
				http.Error(w, "search failed", http.StatusInternalServerError)
				return
			}
			results = append(results, u)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	}
}

func GetUser(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("id")

		var user userResponse

		err := conn.QueryRow(
			`SELECT id, username FROM users WHERE id = $1`,
			userID,
		).Scan(&user.ID, &user.Username)

		if err == sql.ErrNoRows {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		if err != nil {
			http.Error(w, "could not load user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}
