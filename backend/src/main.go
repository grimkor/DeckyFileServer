package main

import (
	"deckyfileserver/logger"
	"deckyfileserver/server"
	"flag"
	"fmt"
	_ "golang.org/x/image/webp"
	"log"
	"os"
)

func main() {
	var rootFolder string
	var port int
	var timeout int
	var verbose bool
	var allowUploads bool
	flag.BoolVar(&verbose, "verbose", false, "log output to stdout (default: false)")
	flag.StringVar(&rootFolder, "f", "/home/david", "Root folder to share")
	flag.IntVar(&port, "p", 8000, "Port number to listen to")
	flag.IntVar(&timeout, "t", 60, "Inactivity timeout (in seconds)")
	flag.BoolVar(&allowUploads, "uploads", false, "Allow uploads from the web page (default: false)")
	flag.Parse()

	logger.SetupLogger("/tmp/deckyfileserver.log", verbose)

	if rootFolder == "" {
		log.Println("[ERROR]: -f flag missing or no value. Please provide a folder eg. `-f /home/deck/`")
		os.Exit(1)
	}
	_, dirErr := os.ReadDir(rootFolder)
	if dirErr != nil {
		log.Println(fmt.Sprintf("[ERROR]: Folder %s cannot be read or does not exist", rootFolder))
		os.Exit(1)
	}
	if port < 1024 || port > 65535 {
		fmt.Println("[ERROR]: Port must be between 1024-65535")
		os.Exit(1)
	}

	s := server.Server{
		Uploads:    allowUploads,
		Port:       port,
		Timeout:    timeout,
		RootFolder: rootFolder,
		UploadJobs: map[string]string{},
	}

	s.Start()
}
