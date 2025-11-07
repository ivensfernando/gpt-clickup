package server

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gpt-clickup/internal/clickup"
	"gpt-clickup/internal/gpt"
	"log"
	"os"
)

func StartServer(port string, logger *logrus.Entry) {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️  No .env file found, using system env")
	}

	openaiKey := os.Getenv("OPENAI_API_KEY")
	clickupKey := os.Getenv("CLICKUP_API_KEY")

	gptClient := gpt.NewClient(openaiKey, logger)
	clickupClient := clickup.NewClient(clickupKey, logger)

	r := gin.Default()

	r.POST("/gpt-clickup", func(c *gin.Context) {
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		response, err := gptClient.Ask(req.Prompt)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		taskID, err := clickupClient.CreateTask(response)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"gpt_response": response,
			"clickup_task": taskID,
		})
	})

	logger.Infof("🚀 Server running on :%s", port)
	r.Run(":" + port)
}
