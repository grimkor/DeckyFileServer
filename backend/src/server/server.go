package server

import (
	"context"
	"crypto/tls"
	"deckyfileserver/thumbnail"
	"embed"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"time"

	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"path/filepath"
	"sort"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

var thumbGen thumbnail.ThumbnailGenerator

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

//go:embed certs/*
var certsFS embed.FS

type DirEntry struct {
	Name      string
	Size      FileSize
	IsDir     bool
	Path      string
	Thumbnail bool
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

func BoolToString(b bool) string {
	if b {
		return "true"
	} else {
		return "false"
	}
}

func sanitiseRequestURI(requestURI string) string {
	if requestURI == "/" {
		requestURI = ""
	}
	return requestURI
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
			Name:      entry.Name(),
			IsDir:     entry.IsDir(),
			Size:      FileSize(info.Size()),
			Path:      path.Join(requestPath, entry.Name()),
			Thumbnail: thumbGen.IsCompatibleType(entry.Name()),
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

type Server struct {
	Unsecure     bool
	Port         int
	Timeout      int
	RootFolder   string
	Server       http.Server
	ShutdownChan chan struct{}
}

func (s *Server) setupHTTPServer() {
	thumbGen = thumbnail.ThumbnailGenerator{
		Cache: thumbnail.Cache{
			Images: map[string][]byte{},
		},
	}

	serveMux := http.NewServeMux()

	connStateCh := make(chan struct{})
	s.ShutdownChan = make(chan struct{})

	cert, _ := certsFS.ReadFile("certs/cert.pem")
	certKey, _ := certsFS.ReadFile("certs/key.pem")

	certPair, _ := tls.X509KeyPair(cert, certKey)
	if s.Unsecure {
		s.Server = http.Server{
			Addr:    fmt.Sprintf(":%v", s.Port),
			Handler: serveMux, ConnState: func(c net.Conn, cs http.ConnState) {
				if cs == http.StateActive {
					connStateCh <- struct{}{}
				}
			}}
	} else {
		s.Server = http.Server{
			Addr: fmt.Sprintf(":%v", s.Port),
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{certPair},
			},
			Handler: serveMux, ConnState: func(c net.Conn, cs http.ConnState) {
				if cs == http.StateActive {
					connStateCh <- struct{}{}
				}
			}}
	}

	go func() {
		timer := time.NewTimer(time.Duration(s.Timeout) * time.Second)
		for {
			select {
			case <-connStateCh:
				timer.Stop()
				timer.Reset(time.Duration(s.Timeout) * time.Second)
			case <-timer.C:
				log.Println("Performing shutdown")
				if err := s.Server.Shutdown(context.Background()); err != nil {
					log.Printf("HTTP Server shutdown: %v", err)
				}
				s.ShutdownChan <- struct{}{}
			}

		}
	}()

	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reverse := r.URL.Query().Get("reverse") == "true"
		showHidden := r.URL.Query().Get("hidden") == "true"
		joinedPath := path.Join(s.RootFolder, r.URL.Path)
		stat, err := os.Stat(joinedPath)
		if err != nil {
			log.Println("endpoint '/':", err.Error())
			return
		}
		if stat.IsDir() {
			dirData, _ := getDir(joinedPath, r.URL.Path, reverse, showHidden)
			if r.Header.Get("HX-Request") == "true" {
				t := template.Must(template.ParseFS(templatesFS, "templates/files.html"))
				t.Execute(w, dirData)
			} else {
				t := template.Must(template.ParseFS(templatesFS, "templates/index.html", "templates/files.html"))
				t.Execute(w, dirData)
			}
		} else {
			filename := path.Base(r.RequestURI)
			w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
			http.ServeFile(w, r, joinedPath)
		}
	})

	serveMux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	serveMux.HandleFunc("/__preview/", func(w http.ResponseWriter, r *http.Request) {
		escaped, escapeErr := url.PathUnescape(r.RequestURI)
		if escapeErr != nil {
			log.Println("endpoint '/__preview/':", escapeErr.Error())
		}
		filePath := strings.TrimPrefix(escaped, "/__preview")
		filePath = path.Join(s.RootFolder, filePath)
		thumb, err := thumbGen.GetThumbnail(filePath)
		if err != nil {
			log.Panicln(err)
			os.Exit(1)
		}
		imaging.Encode(w, thumb, imaging.JPEG)
		//http.ServeFile(w, r, path.Join(thumbGen.ThumbnailDir, thumb))
	})

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
		t := template.Must(template.ParseFS(templatesFS, "templates/menu-items.html"))
		t.Execute(w, templateData)
	})

}

func (s *Server) Start() {
	s.setupHTTPServer()
	if s.Unsecure {
		if err := s.Server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	} else {
		if err := s.Server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}
	<-s.ShutdownChan
}
