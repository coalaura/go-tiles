package gotiles

import (
	"image"
	"image/color"
	"image/draw"
)

func prepareImage(img image.Image) image.Image {
	b := img.Bounds()

	upLeft := image.Point{X: 0, Y: 0}
	lowRight := image.Point{X: b.Max.X + 512, Y: b.Max.Y + 512}
	prepared := image.NewRGBA(image.Rectangle{Min: upLeft, Max: lowRight})

	//col := color.RGBA{R: 221, G: 221, B: 221, A: 255}
	col := color.RGBA{R: 30, G: 30, B: 30, A: 255}

	for x := 0; x < b.Max.X+512; x++ {
		for y := 0; y < b.Max.Y+512; y++ {
			prepared.Set(x, y, col)
		}
	}

	b.Max.Y += 256
	b.Max.X += 256
	b.Min.Y += 256
	b.Min.X += 256

	draw.Draw(prepared, b, img, image.Pt(0, 0), draw.Src)

	return prepared
}
