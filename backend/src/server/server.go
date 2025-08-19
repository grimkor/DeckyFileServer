package server

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"deckyfileserver/thumbnail"
	"embed"
	"encoding/hex"
	"html/template"
	"io"
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

type FilePageData struct {
	Entries      []DirEntry
	Path         string
	ParentPath   string
	IsHome       bool
	Reverse      bool
	ShowHidden   bool
	QueryParams  string
	AllowUploads bool
}

type UploadTemplateData struct {
	Path string
}

func (f FilePageData) ReverseParamText() string {
	str := "?hidden="
	if f.ShowHidden {
		str += "true"
	} else {
		str += "false"
	}
	str += "&reverse="
	if f.Reverse {
		str += "false"
	} else {
		str += "true"
	}
	return str
}

func (f FilePageData) HiddenParamText() string {
	str := "?hidden="
	if f.ShowHidden {
		str += "false"
	} else {
		str += "true"
	}
	str += "&reverse="
	if f.Reverse {
		str += "true"
	} else {
		str += "false"
	}
	return str
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

func getDir(dirPath string, requestPath string, reverseSort bool, showHidden bool, server *Server) (FilePageData, error) {
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
			Thumbnail: !server.DisableThumbnails && thumbGen.IsCompatibleType(entry.Name()),
		})
	}
	sort.Slice(dirs[:], func(i, j int) bool {
		if dirs[i].IsDir != dirs[j].IsDir {
			return dirs[i].IsDir
		}
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) != reverseSort
	})
	queryParams := fmt.Sprintf("?hidden=%s&reverse=%s", BoolToString(showHidden), BoolToString(reverseSort))
	dirData := FilePageData{
		Entries:      dirs,
		Path:         requestPath,
		ParentPath:   parentPath,
		IsHome:       requestPath == "/files/",
		Reverse:      reverseSort,
		ShowHidden:   showHidden,
		QueryParams:  queryParams,
		AllowUploads: server.Uploads,
	}
	return dirData, nil
}

type Server struct {
	Uploads           bool
	DisableThumbnails bool
	Port              int
	Timeout           int
	RootFolder        string
	Server            http.Server
	ShutdownChan      chan struct{}
	UploadJobs        map[string]string
}

func (s *Server) setupHTTPServer() {
	thumbGen = thumbnail.ThumbnailGenerator{
		Cache: thumbnail.Cache{
			Images: map[string]*thumbnail.CacheImageJob{},
			Chans:  map[string]chan *thumbnail.CacheImageJob{},
		},
	}
	thumbGen.SetWorkerCount(4)

	serveMux := http.NewServeMux()

	connStateCh := make(chan struct{})
	s.ShutdownChan = make(chan struct{})

	cert, _ := certsFS.ReadFile("certs/cert.pem")
	certKey, _ := certsFS.ReadFile("certs/key.pem")
	certPair, _ := tls.X509KeyPair(cert, certKey)
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

	go func() {
		timer := time.NewTimer(time.Duration(s.Timeout) * time.Second)
		for {
			select {
			case <-connStateCh:
				timer.Stop()
				timer.Reset(time.Duration(s.Timeout) * time.Second)
			case <-timer.C:
				log.Println("Performing shutdown")
				s.Cleanup()
				if err := s.Server.Shutdown(context.Background()); err != nil {
					log.Printf("[ERROR]: HTTP Server shutdown: %v", err)
				}
				s.ShutdownChan <- struct{}{}
			}

		}
	}()

	serveMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/files/", http.StatusFound)
		}
	})

	serveMux.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		reverse := r.URL.Query().Get("reverse") == "true"
		showHidden := r.URL.Query().Get("hidden") == "true"
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/files")
		joinedPath := path.Join(s.RootFolder, trimmedPath)
		stat, err := os.Stat(joinedPath)
		if err != nil {
			log.Println("[ERROR]: endpoint '/':", err.Error())
			return
		}
		if stat.IsDir() {
			dirData, _ := getDir(joinedPath, r.URL.Path, reverse, showHidden, s)
			var paths []string
			for _, dd := range dirData.Entries {
				paths = append(paths, path.Join(joinedPath, dd.Name))
			}
			if !s.DisableThumbnails {
				go thumbGen.StartBatchJob(paths)
			}
			if r.Header.Get("HX-Request") == "true" {
				t := template.Must(template.ParseFS(templatesFS, "templates/files.html"))
				err := t.ExecuteTemplate(w, "content", dirData)
				if err != nil {
					log.Println(err)
				}
				errMenu := t.ExecuteTemplate(w, "menu", dirData)
				if errMenu != nil {
					log.Println(errMenu)
				}
			} else {
				t := template.Must(template.ParseFS(templatesFS, "templates/index.html", "templates/files.html"))
				err := t.Execute(w, dirData)
				if err != nil {
					log.Println(err)
				}
			}
		} else {
			filename := path.Base(r.RequestURI)
			w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
			http.ServeFile(w, r, joinedPath)
		}
	})

	serveMux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	serveMux.HandleFunc("/preview/", func(w http.ResponseWriter, r *http.Request) {
		escaped, escapeErr := url.PathUnescape(r.RequestURI)
		if escapeErr != nil {
			log.Println("[ERROR]: endpoint '/preview/':", escapeErr.Error())
		}
		filePath := strings.TrimPrefix(escaped, "/preview/files")
		filePath = path.Join(s.RootFolder, filePath)
		thumb, err := thumbGen.GetThumbnail(filePath, r.Context())
		if err != nil {
			log.Println("[ERROR]: /Preview ThumbGen:", err)
		}
		if thumb != nil {
			encodeErr := imaging.Encode(w, thumb, imaging.JPEG)
			if encodeErr != nil {
				log.Println("[ERROR]: error", encodeErr)
			}
		}
	})

	serveMux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if s.Uploads == false {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == "GET" {
			data := UploadTemplateData{
				Path: strings.TrimPrefix(r.URL.Query().Get("path"), "/files"),
			}
			t := template.Must(template.ParseFS(templatesFS, "templates/upload.html"))
			err := t.Execute(w, data)
			if err != nil {
				log.Println(err)
				return
			}
			return
		} else if r.Method == "POST" {
			var existsErr error
			log.Println("[INFO]: endpoint '/upload':", r.URL.Path)
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) //1MB

			start, existsErr := GetHeader(w, r, "Upload-Offset")
			if existsErr != nil {
				return
			}
			uploadIncomplete, existsErr := GetHeader(w, r, "Upload-Incomplete")
			if existsErr != nil {
				return
			}
			checksum, existsErr := GetHeader(w, r, "X-File-Checksum")
			if existsErr != nil {
				return
			}
			fileName, existsErr := GetQueryParam(w, r, "filename")
			if existsErr != nil {
				return
			}
			if result, decodeErr := url.QueryUnescape(fileName); decodeErr != nil {
				log.Println("[ERROR]: endpoint '/upload':", decodeErr)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid file name"))
				return
			} else {
				fileName = result
			}
			directoryPath, existsErr := GetQueryParam(w, r, "path")
			if existsErr != nil {
				return
			}
			cleanPath := path.Clean(path.Join(s.RootFolder, directoryPath))
			tmpFilePath := path.Join(cleanPath, checksum)

			var tmpFile *os.File
			if start == "0" {
				if createdFile, err := CreateFile(w, cleanPath, checksum); err != nil {
					return
				} else {
					tmpFile = createdFile
				}
				s.UploadJobs[checksum] = tmpFilePath
			} else {
				openFile, err := os.OpenFile(tmpFilePath, os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					log.Println("[ERROR]: endpoint '/upload':", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				tmpFile = openFile
			}
			defer tmpFile.Close()

			writeErr := WriteBufferToFile(w, tmpFile, r.Body)
			if writeErr != nil {
				return
			}

			if uploadIncomplete == "false" {
				log.Println("file has been uploaded")
				log.Println("[INFO]: file has been uploaded")
				log.Println("[INFO]: Moving file to:" + filepath.Join(cleanPath, fileName))

				tmpFile.Close()
				checksumErr := CheckAgainstChecksum(w, tmpFilePath, checksum)
				if checksumErr != nil {
					return
				}

				moveErr := os.Rename(tmpFilePath, filepath.Join(cleanPath, fileName))
				if moveErr != nil {
					log.Println("[ERROR]: endpoint '/upload':", moveErr)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				delete(s.UploadJobs, checksum)
			}
			w.WriteHeader(http.StatusOK)

		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	serveMux.HandleFunc("/cancel_upload", func(w http.ResponseWriter, r *http.Request) {
		log.Println("[INFO]: endpoint '/cancel_upload':", r.URL.Path)
		if !s.Uploads {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		filehash := r.URL.Query().Get("filehash")
		existsErr := CheckExists(w, filehash, "filehash")
		if existsErr != nil {
			return
		}
		if path := s.UploadJobs[filehash]; path != "" {
			log.Println("[INFO]: endpoint '/cancel_upload': removing ", path)
			err := os.Remove(path)
			if err != nil {
				log.Println("[ERROR]: endpoint '/cancel_upload':", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			delete(s.UploadJobs, filehash)
		}

	})
}

func (s *Server) Cleanup() {
	for key, value := range s.UploadJobs {
		log.Println(key, value)
		_, statErr := os.Stat(value)
		log.Println("Error", statErr)
		if statErr == nil {
			log.Println("Removing incomplete file: ", value)
			rmErr := os.Remove(value)
			if rmErr != nil {
				log.Println("[ERROR]: Cleanup job: ", rmErr)
				continue 
			}
			delete(s.UploadJobs, key)
		}
	}
}

func (s *Server) Start() {
	s.setupHTTPServer()
	if err := s.Server.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	<-s.ShutdownChan
}

func GetHeader(w http.ResponseWriter, r *http.Request, key string) (string, error) {
	value := r.Header.Get(key)
	err := CheckExists(w, value, key)
	if err != nil {
		return "", err
	}
	return value, nil
}

func GetQueryParam(w http.ResponseWriter, r *http.Request, key string) (string, error) {
	value := r.URL.Query().Get(key)
	err := CheckExists(w, value, key)
	if err != nil {
		return "", err
	}
	return value, nil
}

func CheckExists(w http.ResponseWriter, value string, valueName string) error {
	if value == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("missing param: " + valueName))
		if err != nil {
			log.Println("[ERROR]: endpoint '/upload':", err)
			return err
		}
		return fmt.Errorf("missing param: %s", valueName)
	}
	return nil
}

func CreateFile(w http.ResponseWriter, filePath string, fileName string) (*os.File, error) {
	_, statErr := os.Stat(filePath)
	if os.IsNotExist(statErr) {
		w.WriteHeader(http.StatusBadRequest)
		_, writeErr := w.Write([]byte(fmt.Sprintf("directory path %s does not exist", filePath)))
		if writeErr != nil {
			log.Println("[ERROR]: endpoint '/upload':", writeErr)
		}
	}
	fileCreate, err := os.Create(path.Join(filePath, fileName))
	if err != nil {
		log.Println("[ERROR]: endpoint '/upload_file/':", err)
		w.WriteHeader(http.StatusInternalServerError)
		return &os.File{}, err
	}
	return fileCreate, nil
}

func WriteBufferToFile(w http.ResponseWriter, file *os.File, body io.Reader) error {
	buf := bufio.NewReader(body)
	b := new(bytes.Buffer)
	_, copyErr := io.Copy(b, buf)
	if copyErr != nil {
		log.Println("[ERROR]: endpoint '/upload':", copyErr)
		w.WriteHeader(http.StatusInternalServerError)
		return copyErr
	}
	_, writeErr := file.Write(b.Bytes())
	if writeErr != nil {
		log.Println("[ERROR]: endpoint '/upload_file/':", writeErr)
		w.WriteHeader(http.StatusInternalServerError)
		return writeErr
	}
	return nil
}

func CheckAgainstChecksum(w http.ResponseWriter, filePath string, checksum string) error {
	hash := sha256.New()
	file, _ := os.OpenFile(filePath, os.O_RDONLY, 0644)
	io.Copy(hash, file)
	transferredChecksum := hash.Sum(nil)
	if checksum != hex.EncodeToString(transferredChecksum) {
		log.Println("[ERROR]: endpoint '/upload':", "Checksum mismatch", checksum, hex.EncodeToString(transferredChecksum))
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("Checksum mismatch")
	}
	return nil
}
