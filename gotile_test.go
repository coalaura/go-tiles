package gotile

import (
	"fmt"
	"testing"
)

func TestTileGenerator_Generate(t *testing.T) {
	tg, err := NewTileGenerator("example-map.jpg")
	if err != nil {
		t.Error(err)
		return
	}

	err = tg.Generate(0, 5, TileOptions{
		UseLanczos3: true,
		Verbose:     true,
		JpgQuality:  80,
	})
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Println("Compressing tiles")
	err = tg.CompressTileFolder(false)
	if err != nil {
		t.Error(err)
	}
}
