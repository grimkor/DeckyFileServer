package thumbnail

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"mime"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type CacheImage struct {
	Image image.Image
	Ready bool
}

type Cache struct {
	mu     sync.Mutex
	Images map[string]*CacheImage
}

func (c *Cache) Lock() {
	c.mu.Lock()
}

func (c *Cache) Unlock() {
	c.mu.Unlock()
}

func (m *Cache) IsReady(filePath string) bool {
	m.Lock()
	defer m.Unlock()
	val, ok := m.Images[filePath]
	if ok {
		return val.Ready
	}
	return false
}

func (m *Cache) Add(filePath string, img image.Image) {
	m.Lock()
	defer m.Unlock()
	val, ok := m.Images[filePath]
	if ok && val.Ready {
		return
	}
	if ok {
		val.Image = img
		val.Ready = true
	} else {
		m.Images[filePath] = &CacheImage{Image: img, Ready: true}
	}
}

func (m *Cache) AddPending(filePath string) {
	m.Lock()
	defer m.Unlock()
	_, ok := m.Images[filePath]
	if !ok {
		m.Images[filePath] = &CacheImage{Ready: false}
	}
}

func (m *Cache) Get(filePath string) (*CacheImage, bool) {
	m.Lock()
	defer m.Unlock()
	val, ok := m.Images[filePath]
	return val, ok
}

type ThumbnailGenerator struct {
	ThumbnailDir string
	Cache        Cache
}

func (tg *ThumbnailGenerator) CreateImageThumbnail(filePath string) (image.Image, error) {
	src, e := imaging.Open(filePath)
	if e != nil {
		log.Println("CreateImageThumbnail:", e.Error())
	}
	src = imaging.Resize(src, 128, 0, imaging.NearestNeighbor)
	return src, e
}

func (tg *ThumbnailGenerator) CreateVideoThumbnail(filePath string) (image.Image, error) {
	buf := bytes.NewBuffer(nil)
	err := ffmpeg.Input(filePath).
		Filter("scale", ffmpeg.Args{"128:-1"}).
		Filter("select", ffmpeg.Args{fmt.Sprintf("gte(n,%d)", 0)}).
		Output("pipe:", ffmpeg.KwArgs{"vframes": 1, "format": "image2", "vcodec": "mjpeg", "qscale": 20}).
		WithOutput(buf).
		Run()
	if err != nil {
		return nil, err
	}
	img, error := imaging.Decode(buf)
	return img, error
}

func (tg *ThumbnailGenerator) GetThumbnail(filePath string) (image.Image, error) {
	val, ok := tg.Cache.Get(filePath)
	if ok {
		for !tg.Cache.IsReady(filePath) {
			time.Sleep(10 * time.Millisecond)
		}
		return val.Image, nil
	} else {
		img, err := tg.GenerateThumbnail(filePath)
		if err != nil {
			log.Println("GetThumbnail > GenerateThumbnail", err)
		}

		return img, err
	}
}

func (tg *ThumbnailGenerator) GenerateThumbnail(filePath string) (image.Image, error) {
	tg.Cache.AddPending(filePath)
	ext := mime.TypeByExtension(path.Ext(filePath))
	var img image.Image
	var err error
	if strings.HasPrefix(ext, "image") {
		img, err = tg.CreateImageThumbnail(filePath)
		if err != nil {
			log.Println(err)
		}
		if img != nil {
			tg.Cache.Add(filePath, img)
		}
	} else if strings.HasPrefix(ext, "video") {
		img, err = tg.CreateVideoThumbnail(filePath)
		if err != nil {
			log.Println(err)
		}
		if img != nil {
			tg.Cache.Add(filePath, img)
		}
	}
	return img, err
}

func (tg *ThumbnailGenerator) IsCompatibleType(filePath string) bool {
	mimeType := mime.TypeByExtension(path.Ext(filePath))
	return strings.HasPrefix(mimeType, "image") || strings.HasPrefix(mimeType, "video")
}
