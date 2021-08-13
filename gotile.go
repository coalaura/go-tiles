package gotile

import (
	"errors"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	"gitlab.com/milan44/goquant"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"image"
	_ "image/gif"
	"image/png"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type TileGenerator struct {
	source image.Image
}

type TileOptions struct {
	UseLanczos3   bool
	Verbose       bool
	UseCompressor bool
}

func NewTileGenerator(source string) (*TileGenerator, error) {
	ext := filepath.Ext(source)
	switch ext {
	case ".png", ".bmp", ".tiff", ".jpg", ".jpeg", ".webp":
	default:
		return nil, errors.New("unsupported image format " + ext)
	}

	f, err := os.Open(source)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	return &TileGenerator{
		source: prepareImage(img),
	}, nil
}

func (t *TileGenerator) Generate(minZoom, maxZoom int64, opts TileOptions) error {
	if _, err := os.Stat("tiles"); !os.IsNotExist(err) {
		return errors.New("tiles folder already exists")
	}

	_ = os.MkdirAll("tiles", 0777)

	b := t.source.Bounds()
	if b.Max.X != b.Max.Y {
		return errors.New("source image has to be square")
	}

	start := time.Now()

	p := message.NewPrinter(language.English)
	if opts.Verbose {
		totalCount := int64(0)
		for z := minZoom; z <= maxZoom; z++ {
			totalCount += int64(math.Pow(math.Pow(2, float64(z)), 2))
		}

		fmt.Printf("Generating %s tiles in total (%d zoom levels)\n", p.Sprintf("%d", totalCount), maxZoom-minZoom)
	}

	for z := minZoom; z <= maxZoom; z++ {
		if opts.Verbose {
			count := int64(math.Pow(math.Pow(2, float64(z)), 2))
			fmt.Printf("- Zoom %d/%d (%s tiles)\n", z, maxZoom, p.Sprintf("%d", count))
		}

		err := t.generateZoomLevel(z, opts)
		if err != nil {
			return err
		}
	}

	if opts.Verbose {
		fmt.Printf("Total time generating tiles: %s\n", time.Now().Sub(start).String())
	}

	return nil
}

func (t *TileGenerator) generateZoomLevel(zoom int64, opts TileOptions) error {
	tiles := int64(math.Pow(2, float64(zoom)))
	ogSize := t.source.Bounds().Max.Y - 512
	size := ogSize / int(tiles)

	b := t.source.Bounds()
	if b.Max.X != b.Max.Y {
		return errors.New("source image has to be square")
	}

	if tiles < 1 {
		return errors.New("cannot render zoom level smaller than 0")
	}

	errorChan := make(chan error)
	var countMutex sync.Mutex
	finishedCount := int64(0)
	killAll := false

	var s *spinner.Spinner
	if opts.Verbose {
		s = spinner.New([]string{"-", "/", "|", "\\"}, 250*time.Millisecond)
		s.Prefix = "0% "
		s.HideCursor = true
		s.Start()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if !opts.Verbose {
			wg.Done()
			return
		}

		for {
			time.Sleep(150 * time.Millisecond)

			countMutex.Lock()
			perc := int(math.Round((float64(finishedCount) / float64(tiles*tiles)) * 100))
			s.Prefix = fmt.Sprintf("%d%% ", perc)
			countMutex.Unlock()

			if killAll {
				wg.Done()
				return
			}
		}
	}()

	for x := int64(0); x < tiles; x++ {
		go func(xAxis int64, source image.Image) {
			for y := int64(0); y < tiles; y++ {
				if killAll {
					return
				}

				xx := (int(xAxis) * size) + 256
				yy := (int(y) * size) + 256

				factor := uint(3)
				anchor := 1
				offsetX := 256
				offsetY := 256
				if xAxis == 0 || y == 0 {
					factor = 2

					offsetX = 0
					offsetY = 0
					anchor = 0

					if xAxis == tiles-1 || y == tiles-1 {
						factor = 1
					}
				} else if xAxis == tiles-1 || y == tiles-1 {
					factor = 2
				}

				crop, err := cutter.Crop(source, cutter.Config{
					Width:  size * int(factor),
					Height: size * int(factor),
					Anchor: image.Point{
						X: xx - (size * anchor),
						Y: yy - (size * anchor),
					},
					Mode: cutter.TopLeft,
				})
				if err != nil {
					errorChan <- err
					return
				}

				var tile image.Image
				if opts.UseLanczos3 {
					tile = resize.Resize(256*factor, 256*factor, crop, resize.Lanczos3)
				} else {
					tile = resize.Resize(256*factor, 256*factor, crop, resize.NearestNeighbor)
				}

				crop, err = cutter.Crop(tile, cutter.Config{
					Width:  256,
					Height: 256,
					Anchor: image.Point{
						X: offsetX,
						Y: offsetY,
					},
					Mode: cutter.TopLeft,
				})
				if err != nil {
					errorChan <- err
					return
				}

				err = storeImage(crop, xAxis, y, zoom, opts.UseCompressor)
				if err != nil {
					errorChan <- err
					return
				}

				countMutex.Lock()
				finishedCount++
				countMutex.Unlock()
			}

			errorChan <- nil
		}(x, t.source)
	}

	for {
		err := <-errorChan
		if err != nil {
			killAll = true
			wg.Wait()

			if opts.Verbose {
				s.Stop()
			}

			killAll = true
			return err
		}

		countMutex.Lock()
		if finishedCount == tiles*tiles {
			countMutex.Unlock()

			killAll = true
			wg.Wait()
			if opts.Verbose {
				s.Stop()
			}

			return nil
		}
		countMutex.Unlock()
	}
}

func (t *TileGenerator) CompressTileFolder(verbose bool) error {
	err := os.Chdir("tiles")
	if err != nil {
		return err
	}

	cmd := exec.Command("tar", "-czvf", ".."+string(os.PathSeparator)+"tiles.tar.gz", "*")
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	err = cmd.Run()

	_ = os.Chdir("..")

	return err
}

func storeImage(img image.Image, x, y, z int64, doCompress bool) error {
	file := fmt.Sprintf("tiles/%d/%d/%d.png", z, x, y)
	_ = os.MkdirAll(filepath.Dir(file), 0777)

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()

	if doCompress {
		img, err = goquant.CompressImage(img, &goquant.PNGQuantOptions{
			BinaryLocation: "./lib/pngquant.exe",
		})

		if err != nil {
			return err
		}
	}

	err = png.Encode(out, img)
	if err != nil {
		return err
	}

	return nil
}
