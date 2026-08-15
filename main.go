package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	db "yadegar/internal"
	"yadegar/internal/handlers"
	"yadegar/internal/middleware"

	"github.com/joho/godotenv"
)

func main() {

	// Loading the .env file so we don't expose the passwords lol
	_ = godotenv.Load()
	connString := os.Getenv("DATABASE_URL")

	// connecting to the database
	conn, err := db.Connect(connString)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer conn.Close()

	log.Println("Connected to database successfully")

	// http handlers go here
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(conn))
	mux.HandleFunc("POST /register", handlers.Register(conn))
	mux.HandleFunc("POST /login", handlers.Login(conn))
	mux.HandleFunc("GET /users/search", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.SearchUsers(conn)))
	mux.HandleFunc("POST /events", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.CreateEvent(conn)))
	mux.HandleFunc("GET /events", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ListEvents(conn)))
	mux.HandleFunc("GET /events/{id}", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.GetEvent(conn)))
	mux.HandleFunc("POST /events/{id}/members", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.TagMember(conn)))
	mux.HandleFunc("POST /event-members/{id}/approve", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ApproveMember(conn)))
	mux.HandleFunc("POST /event-members/{id}/reject", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.RejectMember(conn)))
	mux.HandleFunc("DELETE /event-members/{id}", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.RemoveMember(conn)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	mux.HandleFunc("POST /events/{id}/photos", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.UploadPhoto(conn)))
	mux.HandleFunc("GET /gallery", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.Gallery(conn)))
	mux.HandleFunc("GET /notifications", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ListNotifications(conn)))
	mux.HandleFunc("POST /notifications/{id}/read", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ReadNotification(conn)))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS(mux)))
}

// just a test function with a closure
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

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
