package logger

import (
	"io"
	"log"
	"os"
)

func SetupLogger(filepath string, writeToConsole bool) {
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Println("Cannot open log file")
		os.Exit(1)
	}
	if writeToConsole {
		writer := io.MultiWriter(os.Stdout, file)
		log.SetOutput(writer)
	} else {
		log.SetOutput(file)
	}
}
