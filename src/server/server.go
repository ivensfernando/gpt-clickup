package server

import (
	"github.com/gin-gonic/gin"
	"github.com/ivensfernando/gpt-clickup/internal/clickup"
	"github.com/ivensfernando/gpt-clickup/internal/gpt"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
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
	clickupListID := os.Getenv("CLICKUP_LIST_ID")

	missingEnv := gin.H{}
	if openaiKey == "" {
		missingEnv["OPENAI_API_KEY"] = "variável de ambiente obrigatória ausente"
	}
	if clickupKey == "" {
		missingEnv["CLICKUP_API_KEY"] = "variável de ambiente obrigatória ausente"
	}
	if clickupListID == "" {
		missingEnv["CLICKUP_LIST_ID"] = "variável de ambiente obrigatória ausente"
	}

	if len(missingEnv) > 0 {
		logger.WithFields(logrus.Fields(missingEnv)).Fatal("variáveis de ambiente obrigatórias não configuradas")
	}

	gptClient := gpt.NewClient(openaiKey, logger)
	clickupClient := clickup.NewClient(clickupKey, clickupListID, logger)

	r := gin.Default()

	r.POST("/gpt-clickup", func(c *gin.Context) {
		var req struct {
			Prompt string `json:"prompt"`
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

		taskID, err := clickupClient.CreateTask(response)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"gpt_response": response,
			"clickup_task": taskID,
		})
	})

	logger.Infof("🚀 Server running on :%s", port)
	r.Run(":" + port)
}
