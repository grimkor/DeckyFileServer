package thumbnail

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"log"
	"mime"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type Cache struct {
	mu     sync.Mutex
	Images map[string][]byte
}

func (m *Cache) Add(filePath string, img image.Image) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var buf bytes.Buffer
	err := imaging.Encode(&buf, img, imaging.JPEG)
	if err != nil {
		log.Println(err)
	}
	m.Images[filePath] = buf.Bytes()
}

type ThumbnailGenerator struct {
	ThumbnailDir string
	Cache        Cache
}

func (tg *ThumbnailGenerator) HashThumbnailName(filePath string) (string, error) {
	file, err := os.Stat(filePath)
	if err != nil {
		log.Println("HashThumbnailName:", err.Error())
		return "", err
	}
	h := fnv.New32a()
	h.Write([]byte(file.Name() + file.ModTime().String()))
	hashedThumbName := fmt.Sprintf("%d.jpeg", h.Sum32())
	return hashedThumbName, nil
}

func (tg *ThumbnailGenerator) CreateImageThumbnail(filePath string) image.Image {
	src, e := imaging.Open(filePath)
	if e != nil {
		log.Println("CreateImageThumbnail:", e.Error())
	}
	src = imaging.Resize(src, 240, 0, imaging.NearestNeighbor)
	return src
}

func (tg *ThumbnailGenerator) CreateVideoThumbnail(filePath string) image.Image {
	buf := bytes.NewBuffer(nil)
	err := ffmpeg.Input(filePath).
		Filter("scale", ffmpeg.Args{"240:-1"}).
		Filter("select", ffmpeg.Args{fmt.Sprintf("gte(n,%d)", 0)}).
		Output("pipe:", ffmpeg.KwArgs{"vframes": 1, "format": "image2", "vcodec": "mjpeg"}).
		WithOutput(buf).
		Run()
	if err != nil {
		panic(err)
	}
	img, _ := imaging.Decode(buf)
	return img
}

func (tg *ThumbnailGenerator) GetThumbnail(filePath string) (image.Image, error) {
	val, ok := tg.Cache.Images[filePath]
	if ok {
		img, err := imaging.Decode(bytes.NewReader(val))
		if err != nil {
			println("GetThumbnail (decode):", err.Error())
		}
		return img, nil
	}
	img, err := tg.GenerateThumbnail(filePath)
	tg.Cache.Add(filePath, img)
	return img, err
}

func (tg *ThumbnailGenerator) GenerateThumbnail(filePath string) (image.Image, error) {
	ext := mime.TypeByExtension(path.Ext(filePath))

	if strings.HasPrefix(ext, "image") {
		img := tg.CreateImageThumbnail(filePath)
		return img, nil
	}
	if strings.HasPrefix(ext, "video") {
		img := tg.CreateVideoThumbnail(filePath)
		return img, nil
	}
	return nil, nil
}

func (tg *ThumbnailGenerator) IsCompatibleType(filePath string) bool {
	mimeType := mime.TypeByExtension(path.Ext(filePath))
	return strings.HasPrefix(mimeType, "image") || strings.HasPrefix(mimeType, "video")
}
