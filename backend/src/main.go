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
		})
	}
	sort.Slice(dirs[:], func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) != reverseSort
	})
	dirData := Dir{
		dirs,
		requestPath,
		parentPath,
		parentPath == ".",
		reverseSort,
		showHidden,
		fmt.Sprintf("?hidden=%s&reverse=%s", BoolToString(showHidden), BoolToString(reverseSort)),
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
	var rootFolder string
	var port int
	var timeout int
	flag.StringVar(&rootFolder, "f", "", "Root folder to share")
	flag.IntVar(&port, "p", 8000, "Port number to listen to")
	flag.IntVar(&timeout, "t", 60, "Inactivity timeout (in seconds)")
	flag.Parse()

	if rootFolder == "" {
		fmt.Println("-f flag missing or no value. Please provide a folder eg. `-f /home/david/`")
		os.Exit(1)
	}
	_, err := os.ReadDir(rootFolder)
	if err != nil {
		fmt.Println(fmt.Sprintf("Folder %s cannot be read or does not exist", rootFolder))
		os.Exit(1)
	}
	if port < 8000 {
		fmt.Println("Port must be over 8000")
		os.Exit(1)
	}

	m := http.NewServeMux()

	incomeCh := make(chan http.ConnState)

	c, _ := certs.ReadFile("certs/cert.pem")
	k, _ := certs.ReadFile("certs/key.pem")

	cert, err := tls.X509KeyPair(c, k)
	s := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		Handler: m, ConnState: func(c net.Conn, cs http.ConnState) {
			incomeCh <- cs
		}}

	go func() {
		t := time.NewTimer(time.Duration(timeout) * time.Second)
		n := 0
		for {
			select {
			case cs := <-incomeCh:
				if cs == http.StateNew {
					n = n + 1
					t.Stop()
					t.Reset(time.Duration(timeout) * time.Second)
				}
				if cs == http.StateClosed {
					n = n - 1
				}
			case <-t.C:
				if n == 0 {
					fmt.Println("Performing shutdown")
					if err := s.Shutdown(context.Background()); err != nil {
						log.Printf("HTTP Server shutdown: %v", err)
					}
				} else {
					t.Stop()
					t.Reset(10 * time.Second)
				}
			}

		}
	}()

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reverse := r.URL.Query().Get("reverse") == "true"
		showHidden := r.URL.Query().Get("hidden") == "true"
		requestPath := r.URL.Path
		requestPath = sanitiseRequestURI(requestPath)
		folder, _ := strings.CutSuffix(rootFolder, "/")
		path := fmt.Sprintf("%s/%s", folder, requestPath)
		stat, err := os.Stat(path)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		if stat.IsDir() {
			dirData, _ := getDir(path, requestPath, reverse, showHidden)
			if r.Header.Get("HX-Request") == "true" {
				t := template.Must(template.ParseFS(templateFiles, "templates/files.html"))
				t.Execute(w, dirData)
			} else {
				t := template.Must(template.ParseFS(templateFiles, "templates/index.html", "templates/files.html"))
				t.Execute(w, dirData)
			}
		} else {
			http.ServeFile(w, r, path)
		}
	})

	m.Handle("/static/", http.FileServer(http.FS(static)))

	m.HandleFunc("/menu-items", func(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println(fmt.Sprintf("Running on port %v", port))
	if err := s.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
}
