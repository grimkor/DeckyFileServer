package main

import (
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed templates/*
var templateFiles embed.FS

//go:embed static/*
var static embed.FS

//go:embed certs/*
var certs embed.FS

func BoolToString(b bool) string {
	if b {
		return "true"
	} else {
		return "false"
	}
}

type FileSize int64

func (bytes FileSize) FormatSizeUnits() string {
	if bytes >= 1073741824 {
		return fmt.Sprintf("%.2fGB", float64(bytes/1073741824))
	} else if bytes >= 1048576 {
		return fmt.Sprintf("%.2fMB", float64(bytes/1048576))
	} else if bytes >= 1024 {
		return fmt.Sprintf("%.2fKB", float64(bytes/1024))
	} else if bytes > 1 {
		return fmt.Sprintf("%.0f bytes", float64(bytes))
	} else if bytes == 1 {
		return "1 byte"
	} else {
		return "0 bytes"
	}
}

type DirEntry struct {
	Name  string
	Size  FileSize
	IsDir bool
	Path  string
}

type Dir struct {
	Entries     []DirEntry
	Path        string
	ParentPath  string
	IsHome      bool
	Reverse     bool
	ShowHidden  bool
	QueryParams string
}

type MenuItemsData struct {
	Path              string
	Reverse           bool
	ShowHidden        bool
	HiddenParamsText  string
	ReverseParamsText string
}

func getDir(dirPath string, requestPath string, reverseSort bool, showHidden bool) (Dir, error) {
	dirEntry, _ := os.ReadDir(dirPath)
	parentPath := filepath.Dir(requestPath)
	dirs := make([]DirEntry, 0)
	for _, entry := range dirEntry {
		info, _ := entry.Info()
		if !showHidden && strings.HasPrefix(info.Name(), ".") {
			continue
		}
		dirs = append(dirs, DirEntry{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Size:  FileSize(info.Size()),
			Path:  path.Join(requestPath, entry.Name()),
		})
	}
	sort.Slice(dirs[:], func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) != reverseSort
	})
	queryParams := fmt.Sprintf("?hidden=%s&reverse=%s", BoolToString(showHidden), BoolToString(reverseSort))
	dirData := Dir{
		Entries:     dirs,
		Path:        requestPath,
		ParentPath:  parentPath,
		IsHome:      requestPath == "/",
		Reverse:     reverseSort,
		ShowHidden:  showHidden,
		QueryParams: queryParams,
	}
	return dirData, nil
}

func sanitiseRequestURI(requestURI string) string {
	if requestURI == "/" {
		requestURI = ""
	}
	return requestURI
}

func main() {
	file, err := os.OpenFile("/tmp/deckyfileserver.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Println("Cannot open log file")
	} else {
		log.SetOutput(file)
	}
	var rootFolder string
	var port int
	var timeout int
	flag.StringVar(&rootFolder, "f", "", "Root folder to share")
	flag.IntVar(&port, "p", 8000, "Port number to listen to")
	flag.IntVar(&timeout, "t", 60, "Inactivity timeout (in seconds)")
	flag.Parse()

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
	if port < 8000 {
		log.Println("Port must be over 8000")
		os.Exit(1)
	}

	serveMux := http.NewServeMux()

	connStateCh := make(chan struct{})
	shutdownChan := make(chan struct{})

	cert, _ := certs.ReadFile("certs/cert.pem")
	certKey, _ := certs.ReadFile("certs/key.pem")

	certPair, err := tls.X509KeyPair(cert, certKey)
	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certPair},
		},
		Handler: serveMux, ConnState: func(c net.Conn, cs http.ConnState) {
			if cs == http.StateActive {
				connStateCh <- struct{}{}
			}
		}}

	go func() {
		timer := time.NewTimer(time.Duration(timeout) * time.Second)
		for {
			select {
			case <-connStateCh:
				timer.Stop()
				timer.Reset(time.Duration(timeout) * time.Second)
			case <-timer.C:
				log.Println("Performing shutdown")
				if err := server.Shutdown(context.Background()); err != nil {
					log.Printf("HTTP Server shutdown: %v", err)
				}
				shutdownChan <- struct{}{}
			}

		}
	}()

	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reverse := r.URL.Query().Get("reverse") == "true"
		showHidden := r.URL.Query().Get("hidden") == "true"
		joinedPath := path.Join(rootFolder, r.URL.Path)
		stat, err := os.Stat(joinedPath)
		if err != nil {
			log.Println(err.Error())
			return
		}
		if stat.IsDir() {
			dirData, _ := getDir(joinedPath, r.URL.Path, reverse, showHidden)
			if r.Header.Get("HX-Request") == "true" {
				t := template.Must(template.ParseFS(templateFiles, "templates/files.html"))
				t.Execute(w, dirData)
			} else {
				t := template.Must(template.ParseFS(templateFiles, "templates/index.html", "templates/files.html"))
				t.Execute(w, dirData)
			}
		} else {
			filename := path.Base(r.RequestURI)
			w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
			http.ServeFile(w, r, joinedPath)
		}
	})

	serveMux.Handle("/static/", http.FileServer(http.FS(static)))

	serveMux.HandleFunc("/menu-items", func(w http.ResponseWriter, r *http.Request) {
		reverse := r.URL.Query().Get("reverse") == "true"
		showHidden := r.URL.Query().Get("hidden") == "true"
		path := sanitiseRequestURI(r.URL.Query().Get("path"))
		requestPath, _ := strings.CutPrefix(r.URL.Path, "/menu-items")
		requestPath = sanitiseRequestURI(requestPath)
		templateData := MenuItemsData{
			Reverse:           reverse,
			ShowHidden:        showHidden,
			Path:              "/" + strings.TrimLeft(path, "/"),
			HiddenParamsText:  fmt.Sprintf("?hidden=%s&reverse=%s", BoolToString(!showHidden), BoolToString(reverse)),
			ReverseParamsText: fmt.Sprintf("?hidden=%s&reverse=%s", BoolToString(showHidden), BoolToString(!reverse)),
		}
		t := template.Must(template.ParseFS(templateFiles, "templates/menu-items.html"))
		t.Execute(w, templateData)
	})

	log.Println(fmt.Sprintf("Running on port %v", port))
	if err := server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	<-shutdownChan
}
