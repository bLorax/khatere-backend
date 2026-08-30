package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	db "yadegar/internal"
	fsadapter "yadegar/internal/adapters/filesystem"
	httpapi "yadegar/internal/adapters/http"
	pgadapter "yadegar/internal/adapters/postgres"
	appevent "yadegar/internal/application/event"
	appphoto "yadegar/internal/application/photo"
	appuser "yadegar/internal/application/user"
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

	// --- Auth domain wiring (Clean/Hexagonal Architecture) ---
	// The adapter implements the domain port.
	userRepo := pgadapter.NewUserRepository(conn)
	// Each use case takes the port, not the adapter. The use case does not
	// know that Postgres exists.
	registerUC := appuser.NewRegisterUseCase(userRepo)
	loginUC := appuser.NewLoginUseCase(userRepo)
	// The HTTP handler takes the use cases.
	userHandler := httpapi.NewUserHandler(registerUC, loginUC)

	// --- Event domain wiring (Clean/Hexagonal Architecture) ---
	eventRepo := pgadapter.NewEventRepository(conn)
	eventNotifier := pgadapter.NewEventNotifier(conn)
	createEventUC := appevent.NewCreateEventUseCase(eventRepo)
	listEventsUC := appevent.NewListEventsUseCase(eventRepo)
	getEventUC := appevent.NewGetEventUseCase(eventRepo)
	tagMemberUC := appevent.NewTagMemberUseCase(eventRepo, eventNotifier)
	approveMemberUC := appevent.NewApproveMemberUseCase(eventRepo, eventNotifier)
	rejectMemberUC := appevent.NewRejectMemberUseCase(eventRepo, eventNotifier)
	removeMemberUC := appevent.NewRemoveMemberUseCase(eventRepo)

	// --- Photo domain wiring (Clean/Hexagonal Architecture) ---
	photoRepo := pgadapter.NewPhotoRepository(conn)
	photoStorage := fsadapter.NewPhotoStorage("uploads")
	// eventRepo already implements IsApprovedMember, so it satisfies
	// domainphoto.MembershipChecker with no extra adapter code.
	uploadPhotoUC := appphoto.NewUploadPhotoUseCase(photoRepo, photoStorage, eventRepo)
	listEventPhotosUC := appphoto.NewListEventPhotosUseCase(photoRepo, photoStorage)
	photoHandler := httpapi.NewPhotoHandler(uploadPhotoUC)

	eventHandler := httpapi.NewEventHandler(
		createEventUC, listEventsUC, getEventUC,
		tagMemberUC, approveMemberUC, rejectMemberUC, removeMemberUC,
		listEventPhotosUC,
	)

	// http handlers go here
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(conn))
	mux.HandleFunc("POST /register", userHandler.Register)
	mux.HandleFunc("POST /login", userHandler.Login)
	mux.HandleFunc("GET /users/search", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.SearchUsers(conn)))
	mux.HandleFunc("POST /events", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.CreateEvent))
	mux.HandleFunc("GET /events", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.ListEvents))
	mux.HandleFunc("GET /events/{id}", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.GetEvent))
	mux.HandleFunc("POST /events/{id}/members", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.TagMember))
	mux.HandleFunc("POST /event-members/{id}/approve", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.ApproveMember))
	mux.HandleFunc("POST /event-members/{id}/reject", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.RejectMember))
	mux.HandleFunc("DELETE /event-members/{id}", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(eventHandler.RemoveMember))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	mux.HandleFunc("POST /events/{id}/photos", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(photoHandler.UploadPhoto))
	mux.HandleFunc("GET /gallery", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.Gallery(conn)))
	mux.HandleFunc("GET /notifications", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ListNotifications(conn)))
	mux.HandleFunc("POST /notifications/{id}/read", middleware.RequireAuth(os.Getenv("JWT_SECRET"))(handlers.ReadNotification(conn)))
	mux.HandleFunc(
		"GET /users/{id}",
		middleware.RequireAuth(os.Getenv("JWT_SECRET"))(
			handlers.GetUser(conn),
		),
	)

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
