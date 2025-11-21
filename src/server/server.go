package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gpt-clickup/internal/gpt"
	"gpt-clickup/internal/platform/clickup"
	"log"
	"net/http"
	"os"
)

func StartServer(port string, logger *logrus.Entry) {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️  No .env file found, using system env")
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	clickupKey := os.Getenv("CLICKUP_API_KEY")
	// defaultListID := os.Getenv("CLICKUP_LIST_ID") // ID della lista predefinita

	missingEnv := gin.H{}
	if openaiKey == "" {
		missingEnv["OPENAI_API_KEY"] = "required environment variable missing"
	}
	if clickupKey == "" {
		missingEnv["CLICKUP_API_KEY"] = "required environment variable missing"
	}

	if len(missingEnv) > 0 {
		logger.WithFields(logrus.Fields(missingEnv)).Fatal("required environment variables not configured")
	}

	gptClient := gpt.NewClient(openaiKey, logger)
	clickupClient := clickup.NewClient(clickupKey, logger)
	clickupRepo := DefaultRepository()

	if err := ensurePrimaryUser(context.Background(), clickupClient, logger); err != nil {
		logger.WithError(err).Fatal("failed to initialize primary user")
	}
	clickupHandler := NewClickUpHandler(clickupClient, clickupRepo, logger)

	r := gin.Default()

	// endpoint di health check
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	clickupHandler.RegisterRoutes(r)

	gptEndpoint := NewGPTClickUpEndpoint(gptClient, clickupClient, clickupRepo, logger)
	r.POST("/gpt-clickup", gptEndpoint.Handle)

	logger.Infof("🚀 Server running on :%s", port)
	r.Run(":" + port)
}
