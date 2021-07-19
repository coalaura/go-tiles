package gotile

import (
	"errors"
	"fmt"
	"github.com/briandowns/spinner"
	"github.com/nfnt/resize"
	"github.com/oliamb/cutter"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"image"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type TileGenerator struct {
	source image.Image
}

type TileOptions struct {
	UseLanczos3 bool
	Verbose     bool
	JpgQuality  int
}

func NewTileGenerator(source string) (*TileGenerator, error) {
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
		source: img,
	}, nil
}

func (t *TileGenerator) Generate(minZoom, maxZoom int64, opts TileOptions) error {
	if _, err := os.Stat("tiles"); !os.IsNotExist(err) {
		return errors.New("tiles folder already exists")
	}

	if opts.JpgQuality == 0 {
		opts.JpgQuality = 90
	}

	_ = os.MkdirAll("tiles", 0777)

	b := t.source.Bounds()
	if b.Max.X != b.Max.Y {
		return errors.New("source image has to be square")
	}

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

	return nil
}

func (t *TileGenerator) generateZoomLevel(zoom int64, opts TileOptions) error {
	tiles := int64(math.Pow(2, float64(zoom)))
	size := uint(tiles) * 256

	b := t.source.Bounds()
	if b.Max.X != b.Max.Y {
		return errors.New("source image has to be square")
	}

	if tiles < 1 {
		return errors.New("cannot render zoom level smaller than 0")
	}

	var source image.Image
	if opts.UseLanczos3 {
		source = resize.Resize(size, size, t.source, resize.Lanczos3)
	} else {
		source = resize.Resize(size, size, t.source, resize.NearestNeighbor)
	}

	errorChan := make(chan error)
	finishedCount := int64(0)
	killAll := false

	var s *spinner.Spinner
	if opts.Verbose {
		s = spinner.New([]string{"-", "/", "|", "\\"}, 250*time.Millisecond)
		s.Prefix = "0% "
		s.HideCursor = true
		s.Start()
	}

	for x := int64(0); x < tiles; x++ {
		go func(xAxis int64, source image.Image) {
			for y := int64(0); y < tiles; y++ {
				if killAll {
					return
				}

				crop, err := cutter.Crop(source, cutter.Config{
					Width:  256,
					Height: 256,
					Anchor: image.Point{
						X: int(xAxis) * 256,
						Y: int(y) * 256,
					},
					Mode: cutter.TopLeft,
				})
				if err != nil {
					errorChan <- err
					return
				}

				err = storeImage(crop, xAxis, y, zoom, opts.JpgQuality)
				if err != nil {
					errorChan <- err
					return
				}
			}

			errorChan <- nil
		}(x, source)
	}

	for {
		err := <-errorChan
		if err != nil {
			if opts.Verbose {
				s.Stop()
			}

			killAll = true
			return err
		}

		finishedCount++

		if opts.Verbose {
			perc := int(math.Round((float64(finishedCount) / float64(tiles)) * 100))
			s.Prefix = fmt.Sprintf("%d%% ", perc)
		}

		if finishedCount == tiles {
			if opts.Verbose {
				s.Stop()
			}

			return nil
		}
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

func storeImage(img image.Image, x, y, z int64, quality int) error {
	file := fmt.Sprintf("tiles/%d/%d/%d.jpg", z, x, y)
	_ = os.MkdirAll(filepath.Dir(file), 0777)

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, img, &jpeg.Options{
		Quality: quality,
	})
}
