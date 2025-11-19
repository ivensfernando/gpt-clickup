package server

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gpt-clickup/internal/platform/clickup"

	"gpt-clickup/internal/gpt"
	"gpt-clickup/src/model"
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
	//defaultListID := os.Getenv("CLICKUP_LIST_ID")

	missingEnv := gin.H{}
	if openaiKey == "" {
		missingEnv["OPENAI_API_KEY"] = "variável de ambiente obrigatória ausente"
	}
	if clickupKey == "" {
		missingEnv["CLICKUP_API_KEY"] = "variável de ambiente obrigatória ausente"
	}

	if len(missingEnv) > 0 {
		logger.WithFields(logrus.Fields(missingEnv)).Fatal("variáveis de ambiente obrigatórias não configuradas")
	}

	gptClient := gpt.NewClient(openaiKey, logger)
	clickupClient := clickup.NewClient(clickupKey, logger)
	clickupRepo := DefaultRepository()
	clickupHandler := NewClickUpHandler(clickupClient, clickupRepo, logger)

	r := gin.Default()

	// endpoint de health check
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	clickupHandler.RegisterRoutes(r)

	r.POST("/gpt-clickup", func(c *gin.Context) {
		var req struct {
			Prompt string `json:"prompt"`
			ListID string `json:"list_id"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		response, err := gptClient.Ask(req.Prompt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		listID := req.ListID
		//if listID == "" {
		//	listID = defaultListID
		//}
		if listID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "list_id is required"})
			return
		}

		task, err := clickupClient.CreateTask(c.Request.Context(), listID, clickup.TaskRequest{Name: response, Description: "Generated automatically"})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if err := clickupRepo.SaveTasks([]model.TaskClickUp{*task}); err != nil {
			logger.WithError(err).Warn("Failed to persist GPT generated task")
		}

		c.JSON(http.StatusOK, gin.H{
			"gpt_response": response,
			"clickup_task": task.ID,
		})
	})

	logger.Infof("🚀 Server running on :%s", port)
	r.Run(":" + port)
}
