package main

import (
	"deckyfileserver/logger"
	"deckyfileserver/server"
	"flag"
	"fmt"
	"log"
	"os"

	_ "golang.org/x/image/webp"
)

func main() {
	var rootFolder string
	var port int
	var timeout int
	var verbose bool
	var unsecure bool
	flag.BoolVar(&verbose, "verbose", false, "log output to stdout (default: false)")
	flag.StringVar(&rootFolder, "f", "/home/david", "Root folder to share")
	flag.IntVar(&port, "p", 8000, "Port number to listen to")
	flag.IntVar(&timeout, "t", 60, "Inactivity timeout (in seconds)")
	flag.BoolVar(&unsecure, "unsecure", false, "use HTTP instead of HTTPS (default: false)")
	flag.Parse()

	logger.SetupLogger("/tmp/deckyfileserver.log", verbose)

	if rootFolder == "" {
		log.Println("-f flag missing or no value. Please provide a folder eg. `-f /home/deck/`")
		os.Exit(1)
	}
	{
		_, err := os.ReadDir(rootFolder)
		if err != nil {
			log.Println(fmt.Sprintf("Folder %s cannot be read or does not exist", rootFolder))
			os.Exit(1)
		}
	}
	if port < 1024 || port > 65535 {
		fmt.Println("Port must be between 1024-65535")
		os.Exit(1)
	}

	server := server.Server{
		Unsecure:   unsecure,
		Port:       port,
		Timeout:    timeout,
		RootFolder: rootFolder,
	}
	server.Start()
}
