package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	db "yadegar/internal"
	"yadegar/internal/handlers"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	connString := os.Getenv("DATABASE_URL")

	conn, err := db.Connect(connString)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer conn.Close()

	log.Println("Connected to database successfully")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(conn))
	mux.HandleFunc("POST /register", handlers.Register(conn))
	mux.HandleFunc("POST /login", handlers.Login(conn))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))

}

func healthHandler(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count int
		err := conn.QueryRow("SELECT count(*) FROM users").Scan(&count)
		if err != nil {
			http.Error(w, "database query failed", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("ok, users table has rows: "))
		w.Write([]byte(strconv.Itoa(count)))
		w.Write([]byte("\n"))
	}
}
