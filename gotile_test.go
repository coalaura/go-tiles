package gotile

import (
	"fmt"
	"testing"
)

func TestTileGenerator_Generate(t *testing.T) {
	tg, err := NewTileGenerator("map.png")
	if err != nil {
		t.Error(err)
		return
	}

	err = tg.Generate(0, 6, TileOptions{
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
