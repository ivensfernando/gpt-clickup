package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"gpt-clickup/src/db"
	"gpt-clickup/src/server"
	"os"
	"time"
)

var (
	PORT     = os.Getenv("PORT")
	APP_NAME = os.Getenv("APP_NAME")
	log      *logrus.Entry
)

func initLog() {
	// Only log the warning severity or above.
	logrus.SetLevel(logrus.DebugLevel)

	log = logrus.WithFields(logrus.Fields{
		"app": APP_NAME,
	})

}

func init() {
	if PORT == "" {
		PORT = "3010"
	}

}

func main() {

	initLog()
	db.InitDB(log) // ✅ MUST be here before any DB access
	defer handlePanic()

	server.StartServer(PORT, log)
}

func handlePanic() {
	if r := recover(); r != nil {
		log.WithError(fmt.Errorf("%+v", r)).Error(fmt.Sprintf("Application %s panic", APP_NAME))
	}
	//nolint
	time.Sleep(time.Second * 5)
}
