package thumbnail

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"image"
	"log"
	"mime"
	"path"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

//go:embed static/*
var staticFS embed.FS

type CacheImageJob struct {
	Image image.Image
	Ready bool
}

type Cache struct {
	mu     sync.Mutex
	Images map[string]*CacheImageJob
	Chans  map[string]chan *CacheImageJob
}

func (c *Cache) Lock() {
	c.mu.Lock()
}

func (c *Cache) Unlock() {
	c.mu.Unlock()
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
		m.Images[filePath] = &CacheImageJob{Image: img, Ready: true}
	}
	pendingChan, exists := m.Chans[filePath]
	if exists {
		pendingChan <- val
	}
	m.Chans[filePath] = nil
}

func (m *Cache) AddPendingJob(filePath string) {
	m.Lock()
	defer m.Unlock()
	_, ok := m.Images[filePath]
	if !ok {
		m.Images[filePath] = &CacheImageJob{Ready: false}
	}
}

func (m *Cache) Get(filePath string) (*CacheImageJob, bool) {
	m.Lock()
	defer m.Unlock()
	val, ok := m.Images[filePath]
	return val, ok
}

func (m *Cache) AddChan(filePath string, waitChan chan *CacheImageJob) {
	m.Lock()
	defer m.Unlock()
	_, exists := m.Chans[filePath]
	if !exists {
		m.Chans[filePath] = waitChan
	}
}

type ThumbnailGenerator struct {
	ThumbnailDir string
	Cache        Cache
	jobs         chan string
	cancelWork   context.CancelFunc
}

func (tg *ThumbnailGenerator) SetWorkerCount(count int) {
	tg.jobs = make(chan string, count)
	for i := 0; i < count; i++ {
		go tg.work(i, tg.jobs)
	}
}

func (tg *ThumbnailGenerator) work(workerId int, jobs <-chan string) {
	for j := range jobs {
		imageJob, exists := tg.Cache.Get(j)
		if exists && imageJob.Ready {
			continue
		}
		_, err := tg.GenerateThumbnail(j)
		if err != nil {
			log.Println("[ERROR]: work => Error: ", workerId, j, err)
		}
	}
}

func (tg *ThumbnailGenerator) StartBatchJob(paths []string) {
	if tg.cancelWork != nil {
		tg.cancelWork()
	}
	ctx, cancel := context.WithCancel(context.Background())
	tg.cancelWork = cancel
	for _, p := range paths {
		if !tg.IsCompatibleType(p) {
			continue
		}
		select {
		case <-ctx.Done():
			break
		case tg.jobs <- p:
			break
		}
	}
}

func (tg *ThumbnailGenerator) CreateImageThumbnail(filePath string) (*image.NRGBA, error) {
	src, err := imaging.Open(filePath)
	if err != nil {
		log.Println("[ERROR]: CreateImageThumbnail => imaging.Open()", filePath, err.Error())
		return nil, err
	}
	// resized := imaging.Resize(src, 128, 0, imaging.NearestNeighbor)
	resized := imaging.Thumbnail(src, 128, 128, imaging.NearestNeighbor)
	return resized, err
}

func (tg *ThumbnailGenerator) CreateVideoThumbnail(filePath string) (image.Image, error) {
	buf := bytes.NewBuffer(nil)
	err := ffmpeg.Input(filePath).
		Filter("scale", ffmpeg.Args{"128:-1"}).
		Filter("select", ffmpeg.Args{"gte(n,0)"}).
		Output("pipe:", ffmpeg.KwArgs{"vframes": 1, "format": "image2", "vcodec": "mjpeg", "qscale": 20}).
		WithOutput(buf).
		Run()
	if err != nil {
		log.Println("[ERROR]: ThumbnailGenerator => CreateVideoThumbnail => ffmpeg.Run()", filePath, err)
		return nil, err
	}
	img, error := imaging.Decode(buf)
	if error != nil {
		log.Println("[ERROR]: ThumbnailGenerator => CreateVideoThumbnail => imaging.Decode()", filePath, error)
	}
	return img, error
}

func (m *Cache) WaitForThumbnail(filePath string, requestContext context.Context) (image.Image, error) {
	imageJob, ok := m.Get(filePath)
	if ok {
		if imageJob.Ready {
			return imageJob.Image, nil
		}
		waitChan, _ := m.Chans[filePath]
		if waitChan == nil {
			waitChan = make(chan *CacheImageJob)
		}
		m.AddChan(filePath, waitChan)
		select {
		case result := <-waitChan:
			return result.Image, nil
		case <-requestContext.Done():
		}
	}
	return nil, errors.New("Problem waiting for thumbnail")
}

func (tg *ThumbnailGenerator) GetThumbnail(filePath string, requestContext context.Context) (image.Image, error) {
	imageJob, ok := tg.Cache.Get(filePath)
	if ok {
		if !imageJob.Ready {
			img, err := tg.Cache.WaitForThumbnail(filePath, requestContext)
			if err != nil {
				log.Println("[ERROR]: GetThumbnail => WaitForThumbnail()", filePath, err)
			}
			return img, err
		}
		return imageJob.Image, nil
	} else {
		tg.Cache.AddPendingJob(filePath)
		img, err := tg.Cache.WaitForThumbnail(filePath, requestContext)
		if err != nil {
			log.Println("[ERROR]: GetThumbnail => WaitForThumbnail()", filePath, err)
		}
		return img, err
	}
}

func (tg *ThumbnailGenerator) GenerateThumbnail(filePath string) (image.Image, error) {
	tg.Cache.AddPendingJob(filePath)
	ext := mime.TypeByExtension(path.Ext(filePath))
	if strings.HasPrefix(ext, "image") {
		img, err := tg.CreateImageThumbnail(filePath)
		if err != nil {
			img = nil
			log.Println("[ERROR]: GenerateThumbnail => CreateImageThumbnail()", filePath, img, err)
			tg.Cache.Add(filePath, nil)
		}
		if img != nil {
			tg.Cache.Add(filePath, img)
		}
		return img, err
		// if img != nil {
		// 	tg.Cache.Add(filePath, img)
		// }
	} else if strings.HasPrefix(ext, "video") {
		img, err := tg.CreateVideoThumbnail(filePath)
		if err != nil {
			log.Println("GenerateThumbnail => ERROR", filePath, err)
		}
		if img != nil {
			tg.Cache.Add(filePath, img)
		}
		return img, err
	}
	// if err != nil {
	// 	log.Println("[ERROR]: GenerateThumbnail", err)
	// 	return nil, err
	// }
	return nil, errors.New("Request to generate thumbnail but not image/video")
}

func (tg *ThumbnailGenerator) IsCompatibleType(filePath string) bool {
	mimeType := mime.TypeByExtension(path.Ext(filePath))
	return strings.HasPrefix(mimeType, "image") || strings.HasPrefix(mimeType, "video")
}
