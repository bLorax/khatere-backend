package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	db "yadegar/internal"
	fsadapter "yadegar/internal/adapters/filesystem"
	httpapi "yadegar/internal/adapters/http"
	kafkaadapter "yadegar/internal/adapters/kafka"
	pgadapter "yadegar/internal/adapters/postgres"
	redisadapter "yadegar/internal/adapters/redis"
	s3adapter "yadegar/internal/adapters/s3"
	"yadegar/internal/adapters/thumbnailqueue"
	appevent "yadegar/internal/application/event"
	appgallery "yadegar/internal/application/gallery"
	appnotif "yadegar/internal/application/notification"
	appphoto "yadegar/internal/application/photo"
	appuser "yadegar/internal/application/user"
	domainphoto "yadegar/internal/domain/photo"
	"yadegar/internal/middleware"
	"yadegar/internal/telemetry"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {

	// Loading the .env file so we don't expose the passwords lol
	_ = godotenv.Load()
	connString := os.Getenv("DATABASE_URL")
	jwtSecret := os.Getenv("JWT_SECRET")

	// connecting to the database
	conn, err := db.Connect(connString)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	log.Println("Connected to database successfully")

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient, err := redisadapter.Connect(redisAddr)
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	log.Println("Connected to Redis successfully")

	ctx := context.Background()

	otelShutdown, err := telemetry.Init(ctx)
	if err != nil {
		log.Fatalf("failed to initialize OpenTelemetry: %v", err)
	}
	// --- Auth domain wiring (Clean/Hexagonal Architecture) ---
	// The adapter implements the domain port.
	userRepo := pgadapter.NewUserRepository(conn)
	// Each use case takes the port, not the adapter. The use case does not
	// know that Postgres exists.
	registerUC := appuser.NewRegisterUseCase(userRepo)
	loginUC := appuser.NewLoginUseCase(userRepo)
	searchUsersUC := appuser.NewSearchUsersUseCase(userRepo)
	getUserUC := appuser.NewGetUserUseCase(userRepo)
	// The HTTP handler takes the use cases.
	userHandler := httpapi.NewUserHandler(registerUC, loginUC, searchUsersUC, getUserUC)

	// --- Event domain wiring (Clean/Hexagonal Architecture) ---
	eventRepo := pgadapter.NewEventRepository(conn)
	cachedEventRepo := redisadapter.NewCachedEventRepository(eventRepo, redisClient, 60*time.Second)
	notifRepo := pgadapter.NewNotificationRepository(conn)
	kafkaBrokers := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if os.Getenv("KAFKA_BROKERS") == "" {
		kafkaBrokers = []string{"localhost:9092"}
	}
	kafkaWriter := kafkaadapter.NewWriter(kafkaBrokers)
	photoNotifier := kafkaadapter.NewPhotoNotifier(kafkaWriter)

	notificationConsumer := kafkaadapter.NewNotificationConsumer(kafkaBrokers, "khatere-notifications", notifRepo, eventRepo)
	notificationConsumer.Start(context.Background())

	outboxPublisher := kafkaadapter.NewOutboxPublisher(conn, kafkaWriter)
	outboxPublisher.Start(context.Background())

	createEventUC := appevent.NewCreateEventUseCase(cachedEventRepo)
	listEventsUC := appevent.NewListEventsUseCase(cachedEventRepo)
	photoRepo := pgadapter.NewPhotoRepository(conn)
	var photoStorage domainphoto.Storage
	switch os.Getenv("STORAGE_BACKEND") {
	case "s3":
		s3Client, err := s3adapter.Connect(
			os.Getenv("S3_ENDPOINT"),
			os.Getenv("S3_REGION"),
			os.Getenv("S3_ACCESS_KEY"), // empty → falls back to IAM role
			os.Getenv("S3_SECRET_KEY"),
			os.Getenv("S3_BUCKET"),
			os.Getenv("S3_USE_SSL") != "false", // SSL on by default
		)
		if err != nil {
			log.Fatalf("could not connect to object storage: %v", err)
		}
		presignExpiry := 15 * time.Minute
		photoStorage = s3adapter.NewPhotoStorage(s3Client, os.Getenv("S3_BUCKET"), presignExpiry)
		log.Println("photo storage: S3 (" + os.Getenv("S3_BUCKET") + ")")
	default:
		photoStorage = fsadapter.NewPhotoStorage("uploads")
		log.Println("photo storage: local filesystem")
	}
	listEventPhotosUC := appphoto.NewListEventPhotosUseCase(photoRepo, photoStorage)
	getEventUC := appevent.NewGetEventUseCase(cachedEventRepo, listEventPhotosUC)
	tagMemberUC := appevent.NewTagMemberUseCase(cachedEventRepo)
	approveMemberUC := appevent.NewApproveMemberUseCase(cachedEventRepo)
	rejectMemberUC := appevent.NewRejectMemberUseCase(cachedEventRepo)
	removeMemberUC := appevent.NewRemoveMemberUseCase(cachedEventRepo)

	// --- Photo domain wiring (Clean/Hexagonal Architecture) ---
	// eventRepo already implements IsApprovedMember, so it satisfies
	// domainphoto.MembershipChecker with no extra adapter code.
	thumbnailWorkers := 4
	if v := os.Getenv("THUMBNAIL_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			thumbnailWorkers = n
		}
	}
	thumbnailQueueSize := 100
	if v := os.Getenv("THUMBNAIL_QUEUE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			thumbnailQueueSize = n
		}
	}
	thumbnailPool := thumbnailqueue.NewPool(photoStorage, thumbnailWorkers, thumbnailQueueSize)
	telemetry.RegisterThumbnailQueueDepth(thumbnailPool.QueueDepth)
	uploadPhotoUC := appphoto.NewUploadPhotoUseCase(photoRepo, photoStorage, cachedEventRepo, thumbnailPool, photoNotifier)
	photoHandler := httpapi.NewPhotoHandler(uploadPhotoUC)

	// --- Notification domain wiring (Clean/Hexagonal Architecture) ---
	listNotificationsUC := appnotif.NewListNotificationsUseCase(notifRepo)
	readNotificationUC := appnotif.NewReadNotificationUseCase(notifRepo)
	notificationHandler := httpapi.NewNotificationHandler(listNotificationsUC, readNotificationUC)

	// --- Gallery read-model wiring (Clean/Hexagonal Architecture) ---
	galleryRepo := pgadapter.NewGalleryRepository(conn)
	cachedGalleryRepo := redisadapter.NewCachedGalleryRepository(galleryRepo, redisClient, 30*time.Second)
	listGalleryUC := appgallery.NewListGalleryUseCase(cachedGalleryRepo)
	galleryHandler := httpapi.NewGalleryHandler(listGalleryUC)

	eventHandler := httpapi.NewEventHandler(
		createEventUC, listEventsUC, getEventUC,
		tagMemberUC, approveMemberUC, rejectMemberUC, removeMemberUC,
		listEventPhotosUC,
	)

	// http handlers go here
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(conn, redisClient))
	mux.HandleFunc("POST /register", telemetry.InstrumentRoute("POST /register", "POST", userHandler.Register))
	mux.HandleFunc("POST /login", telemetry.InstrumentRoute("POST /login", "POST", middleware.RateLimit(redisClient, 5, 15*time.Minute)(userHandler.Login)))
	mux.HandleFunc("GET /users/search", telemetry.InstrumentRoute("GET /users/search", "GET", middleware.RequireAuth(jwtSecret)(userHandler.SearchUsers)))
	mux.HandleFunc("POST /events", telemetry.InstrumentRoute("POST /events", "POST", middleware.RequireAuth(jwtSecret)(eventHandler.CreateEvent)))
	mux.HandleFunc("GET /events", telemetry.InstrumentRoute("GET /events", "GET", middleware.RequireAuth(jwtSecret)(eventHandler.ListEvents)))
	mux.HandleFunc("GET /events/{id}", telemetry.InstrumentRoute("GET /events/{id}", "GET", middleware.RequireAuth(jwtSecret)(eventHandler.GetEvent)))
	mux.HandleFunc("POST /events/{id}/members", telemetry.InstrumentRoute("POST /events/{id}/members", "POST", middleware.RequireAuth(jwtSecret)(eventHandler.TagMember)))
	mux.HandleFunc("POST /event-members/{id}/approve", telemetry.InstrumentRoute("POST /event-members/{id}/approve", "POST", middleware.RequireAuth(jwtSecret)(eventHandler.ApproveMember)))
	mux.HandleFunc("POST /event-members/{id}/reject", telemetry.InstrumentRoute("POST /event-members/{id}/reject", "POST", middleware.RequireAuth(jwtSecret)(eventHandler.RejectMember)))
	mux.HandleFunc("DELETE /event-members/{id}", telemetry.InstrumentRoute("DELETE /event-members/{id}", "DELETE", middleware.RequireAuth(jwtSecret)(eventHandler.RemoveMember)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))
	mux.HandleFunc("POST /events/{id}/photos", telemetry.InstrumentRoute("POST /events/{id}/photos", "POST", middleware.RequireAuth(jwtSecret)(photoHandler.UploadPhoto)))
	mux.HandleFunc("GET /gallery", telemetry.InstrumentRoute("GET /gallery", "GET", middleware.RequireAuth(jwtSecret)(galleryHandler.List)))
	mux.HandleFunc("GET /notifications", telemetry.InstrumentRoute("GET /notifications", "GET", middleware.RequireAuth(jwtSecret)(notificationHandler.ListNotifications)))
	mux.HandleFunc("POST /notifications/{id}/read", telemetry.InstrumentRoute("POST /notifications/{id}/read", "POST", middleware.RequireAuth(jwtSecret)(notificationHandler.ReadNotification)))
	mux.HandleFunc(
		"GET /users/{id}",
		telemetry.InstrumentRoute("GET /users/{id}", "GET",
			middleware.RequireAuth(jwtSecret)(
				userHandler.GetUser,
			),
		),
	)
	mux.Handle("GET /metrics", telemetry.MetricsHandler())

	log.Println("listening on :8080")

	handler := otelhttp.NewHandler(withCORS(mux), "khatere-http")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Run the server in the background so main can wait for a shutdown signal.
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for SIGINT (Ctrl+C) or SIGTERM (e.g. from Docker/Kubernetes).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")

	// 1. Stop accepting new HTTP requests. Existing in-flight requests get
	// up to 10 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// 2. Drain the thumbnail worker pool. No new HTTP requests can enqueue
	// jobs anymore, so this only waits for jobs already in flight or queued.
	thumbnailPool.Shutdown()
	log.Println("thumbnail pool drained")

	// 3. Close the Redis client.
	if err := redisClient.Close(); err != nil {
		log.Printf("redis close error: %v", err)
	}
	log.Println("redis connection closed")

	outboxPublisher.Shutdown()
	log.Println("outbox publisher stopped")
	notificationConsumer.Shutdown()
	log.Println("kafka consumer stopped")

	if err := kafkaWriter.Close(); err != nil {
		log.Printf("kafka writer close error: %v", err)
	}
	log.Println("kafka producer closed")

	// 4. Close the Postgres connection.
	if err := conn.Close(); err != nil {
		log.Printf("postgres close error: %v", err)
	}
	log.Println("postgres connection closed")

	// 5. Shut down OpenTelemetry last, so it can flush any spans from the
	// steps above.
	if err := otelShutdown(context.Background()); err != nil {
		log.Printf("failed to shutdown OpenTelemetry: %v", err)
	}

	log.Println("shutdown complete")
}

// just a test function with a closure
func healthHandler(conn *sql.DB, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var count int
		err := conn.QueryRow("SELECT count(*) FROM users").Scan(&count)
		if err != nil {
			http.Error(w, "database query failed", http.StatusInternalServerError)
			return
		}

		redisStatus := "ok"
		if err := redisClient.Ping(r.Context()).Err(); err != nil {
			redisStatus = "fail"
		}

		w.Write([]byte("ok, users table has rows: "))
		w.Write([]byte(strconv.Itoa(count)))
		w.Write([]byte("\nredis: "))
		w.Write([]byte(redisStatus))
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
