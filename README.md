# Gotiles
Map tile generator for tiling square map images.

## Usage

```go
package main

import "gitlab.com/milan44/gotiles"

func main() {
    // Create a new tile generator and load an image
    tg, err := gotile.NewTileGenerator("my-cool-map.png")
    if err != nil {
        panic(err)
    }
    
    // Generate map tiles -> ./tiles/{z}/{x}/{y}.jpg
    err = tg.Generate(0, 6, gotile.TileOptions{
        UseLanczos3: true, // Use Lanczos3 or NearestNeighbor for resizing (Lanczos3 is better if tiles have to get upscaled)
        Verbose:     true, // Print progress information to stdout
        JpgQuality:  80, // Quality for generated jpg tiles (default: 90)
    })
    if err != nil {
        panic(err)
    }
    
    // Optionally you can create a .tar.gz file of the tiles directory (requires tar to be installed)
    err = tg.CompressTileFolder(false /* Verbose parameter same as above */)
    if err != nil {
        panic(err)
    }
}
```