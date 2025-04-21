package gotiles

import (
	"fmt"
	"testing"
)

func TestTileGenerator_Generate(t *testing.T) {
	tg, err := NewTileGenerator("map.png", TileOptions{
		UseLanczos3:   true,
		Verbose:       true,
		UseCompressor: true,
	})
	if err != nil {
		panic(err)
	}

	err = tg.Generate(0, 8)
	if err != nil {
		panic(err)
	}

	fmt.Println("Compressing tiles")

	err = tg.CompressTileFolder(false)
	if err != nil {
		panic(err)
	}
}
