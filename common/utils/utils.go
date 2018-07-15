package utils

import (
	"fmt"
	"log"
	"os"
)

var Logger *log.Logger

func InitLogger() {
	Logger = log.New(os.Stdout, "", 0)
	Logger.SetPrefix("")

}

func LogToConsole(message string) {
	Logger.Printf("\r\033[0K%s", message)
	Logger.Printf("\r\033[0KCommand: ")
}

func FailOnError(err error, msg string) {
	if err != nil {
		Logger.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}
