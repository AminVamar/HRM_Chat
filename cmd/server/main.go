package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chat-backend/config"
	"chat-backend/docs"
	deliveryHTTP "chat-backend/internal/delivery/http"
	"chat-backend/internal/delivery/ws"
	"chat-backend/internal/repository/postgres"
	"chat-backend/internal/usecase"
	"chat-backend/pkg/crypto"
)

const messageRetentionDays = 30

// @title           Чат для Системы HRM
// @version         1.0

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey LoginHeaderAuth
// @in header
// @name Login
// @description Передайте имя пользователя или email для авторизации (например: amin, fed, ernest или far@gmail.com)
func main() {
	cfg := config.LoadConfig()
	ctx := context.Background()

	// Адрес, на который Swagger отправляет запросы. На сервере задаётся через SWAGGER_HOST.
	if host := os.Getenv("SWAGGER_HOST"); host != "" {
		docs.SwaggerInfo.Host = host
	} else {
		docs.SwaggerInfo.Host = "localhost:" + cfg.Port
	}

	usingDefaultKey, err := crypto.Init()
	if err != nil {
		log.Fatalf("Ошибка ключа шифрования: %v", err)
	}
	if usingDefaultKey {
		log.Println("ВНИМАНИЕ: ENCRYPTION_KEY не задан, используется встроенный ключ — сообщения фактически не защищены")
	}

	pg, err := postgres.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}
	defer pg.Close()

	if err := pg.AutoMigrate(ctx, "./migrations/000001_init.up.sql"); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	userRepo := postgres.NewUserRepository(pg.Pool)
	chatRepo := postgres.NewChatRepository(pg.Pool)
	messageRepo := postgres.NewMessageRepository(pg.Pool)
	fileRepo := postgres.NewFileRepository(pg.Pool)
	reactionRepo := postgres.NewReactionRepository(pg.Pool)

	userUC := usecase.NewUserUseCase(userRepo, cfg.UploadDir)
	messageUC := usecase.NewMessageUseCase(messageRepo, chatRepo, reactionRepo)
	chatUC := usecase.NewChatUseCase(chatRepo, userRepo, messageRepo, cfg.UploadDir)
	fileUC := usecase.NewFileUseCase(fileRepo, chatRepo, messageUC, cfg.UploadDir)

	hub := ws.NewHub()
	go hub.Run()

	notifier := ws.NewNotifier(hub, chatRepo)

	userHandler := deliveryHTTP.NewUserHandler(userUC, hub)
	chatHandler := deliveryHTTP.NewChatHandler(chatUC, fileUC, hub, notifier)
	messageHandler := deliveryHTTP.NewMessageHandler(messageUC, fileUC, notifier)
	fileHandler := deliveryHTTP.NewFileHandler(fileUC)
	globalHandler := deliveryHTTP.NewGlobalHandler(chatUC, messageUC)
	wsHandler := ws.NewWSHandler(hub, userUC, messageUC, chatUC, notifier)
	authMiddleware := deliveryHTTP.NewAuthMiddleware(userUC)

	router := deliveryHTTP.NewRouter(
		userHandler,
		chatHandler,
		messageHandler,
		fileHandler,
		globalHandler,
		wsHandler,
		authMiddleware,
	)

	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	go func() {
		runCleanup := func() {
			n, orphanPaths, err := messageRepo.DeleteOlderThan(cleanupCtx, time.Now().AddDate(0, 0, -messageRetentionDays))
			if err != nil {
				log.Printf("Ошибка очистки старых сообщений: %v", err)
				return
			}
			if n > 0 {
				log.Printf("Очистка: удалено сообщений старше %d дней: %d", messageRetentionDays, n)
			}

			for _, path := range orphanPaths {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					log.Printf("Ошибка удаления файла %s: %v", path, err)
				}
			}
			if len(orphanPaths) > 0 {
				log.Printf("Очистка: удалено файлов с диска: %d", len(orphanPaths))
			}
		}

		runCleanup()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runCleanup()
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      deliveryHTTP.CORS(router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Сервер запущен на порту %s", cfg.Port)
		log.Printf("Swagger документация доступна по адресу: http://localhost:%s/swagger/index.html", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка работы сервера: %v", err)
		}
	}()

	<-shutdown
	log.Println("Завершение работы сервера...")
	cancelCleanup()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Принудительное завершение работы сервера: %v", err)
	}

	log.Println("Сервер успешно остановлен")
}
