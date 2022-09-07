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
	opts   TileOptions
}

type TileOptions struct {
	UseLanczos3             bool
	Verbose                 bool
	UseCompressor           bool
	IgnoreCompressionErrors bool
}

func NewTileGenerator(source string, opts TileOptions) (*TileGenerator, error) {
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

	if opts.UseLanczos3 {
		img = prepareImage(img)
	}

	return &TileGenerator{
		source: img,
		opts:   opts,
	}, nil
}

func (t *TileGenerator) Generate(minZoom, maxZoom int64) error {
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
	if t.opts.Verbose {
		totalCount := int64(0)
		for z := minZoom; z <= maxZoom; z++ {
			totalCount += int64(math.Pow(math.Pow(2, float64(z)), 2))
		}

		fmt.Printf("Generating %s tiles in total (%d zoom levels)\n", p.Sprintf("%d", totalCount), maxZoom-minZoom)
	}

	for z := minZoom; z <= maxZoom; z++ {
		if t.opts.Verbose {
			count := int64(math.Pow(math.Pow(2, float64(z)), 2))
			fmt.Printf("- Zoom %d/%d (%s tiles)\n", z, maxZoom, p.Sprintf("%d", count))
		}

		err := t.generateZoomLevel(z)
		if err != nil {
			return err
		}
	}

	if t.opts.Verbose {
		fmt.Printf("Total time generating tiles: %s\n", time.Now().Sub(start).String())
	}

	return nil
}

func (t *TileGenerator) generateZoomLevel(zoom int64) error {
	tiles := int64(math.Pow(2, float64(zoom)))
	ogSize := t.source.Bounds().Max.Y - 512
	if !t.opts.UseLanczos3 {
		ogSize += 512
	}
	rawSize := float64(ogSize) / float64(tiles)
	size := int(math.Round(rawSize))

	b := t.source.Bounds()
	if b.Max.X != b.Max.Y {
		return errors.New("source image has to be square")
	}

	if tiles < 1 {
		return errors.New("cannot render zoom level smaller than 1")
	}

	errorChan := make(chan error)
	var countMutex sync.Mutex
	finishedCount := int64(0)
	killAll := false

	var s *spinner.Spinner
	if t.opts.Verbose {
		s = spinner.New([]string{"-", "/", "|", "\\"}, 250*time.Millisecond)
		s.Prefix = "0% "
		s.HideCursor = true
		s.Start()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		if !t.opts.Verbose {
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

				if t.opts.UseLanczos3 {
					xx := rnd(float64(xAxis)*rawSize) + 256
					yy := rnd(float64(y)*rawSize) + 256

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
						Width:  rnd(rawSize * float64(factor)),
						Height: rnd(rawSize * float64(factor)),
						Anchor: image.Point{
							X: xx - rnd(rawSize*float64(anchor)),
							Y: yy - rnd(rawSize*float64(anchor)),
						},
						Mode: cutter.TopLeft,
					})
					if err != nil {
						errorChan <- err
						return
					}

					tile := resize.Resize(256*factor, 256*factor, crop, resize.Lanczos3)

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

					err = storeImage(crop, xAxis, y, zoom, t.opts.UseCompressor, t.opts.IgnoreCompressionErrors)
					if err != nil {
						errorChan <- err
						return
					}
				} else {
					crop, err := cutter.Crop(source, cutter.Config{
						Width:  size,
						Height: size,
						Anchor: image.Point{
							X: rnd(float64(xAxis) * rawSize),
							Y: rnd(float64(y) * rawSize),
						},
						Mode: cutter.TopLeft,
					})
					if err != nil {
						errorChan <- err
						return
					}

					tile := resize.Resize(256, 256, crop, resize.NearestNeighbor)

					err = storeImage(tile, xAxis, y, zoom, t.opts.UseCompressor, t.opts.IgnoreCompressionErrors)
					if err != nil {
						fmt.Println("2")
						errorChan <- err
						return
					}
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

			if t.opts.Verbose {
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
			if t.opts.Verbose {
				s.Stop()
			}

			return nil
		}
		countMutex.Unlock()
	}
}

func rnd(f float64) int {
	return int(math.Floor(f))
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

func storeImage(img image.Image, x, y, z int64, doCompress, ignoreCompressionErrors bool) error {
	file := fmt.Sprintf("tiles/%d/%d/%d.png", z, x, y)
	_ = os.MkdirAll(filepath.Dir(file), 0777)

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()

	if doCompress {
		cImg, err := goquant.CompressImage(img, &goquant.PNGQuantOptions{
			BinaryLocation: "./lib/pngquant.exe",
		})

		if err != nil {
			if !ignoreCompressionErrors {
				return err
			}
		} else {
			img = cImg
		}
	}

	err = png.Encode(out, img)
	if err != nil {
		return err
	}

	return nil
}
