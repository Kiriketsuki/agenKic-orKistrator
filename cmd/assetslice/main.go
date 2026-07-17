// Command assetslice cuts the design-pack strip textures into standalone
// tile PNGs. Godot's texture-repeat cannot wrap a sub-region of an atlas,
// so any tile used as a repeating fill (floor plane, wall band) needs its
// own file. Names follow godot/assets/asset_manifest.json tile order.
package main

import (
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

var tileNames = []string{
	"stone_floor", "stone_cracked", "stone_wall", "wall_moss",
	"moss_overlay", "rune_floor", "stairs", "wood_floor",
}

func main() {
	src := filepath.Join("godot", "assets", "tiles", "tower_tileset.png")
	f, err := os.Open(src)
	if err != nil {
		log.Fatalf("open %s: %v", src, err)
	}
	img, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		log.Fatalf("decode %s: %v", src, err)
	}

	outDir := filepath.Join("godot", "assets", "tiles", "sliced")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	si, ok := img.(subImager)
	if !ok {
		log.Fatalf("image type %T lacks SubImage", img)
	}

	for i, name := range tileNames {
		tile := si.SubImage(image.Rect(i*16, 0, (i+1)*16, 16))
		outPath := filepath.Join(outDir, name+".png")
		out, err := os.Create(outPath)
		if err != nil {
			log.Fatalf("create %s: %v", outPath, err)
		}
		if err := png.Encode(out, tile); err != nil {
			log.Fatalf("encode %s: %v", outPath, err)
		}
		_ = out.Close()
		log.Printf("wrote %s", outPath)
	}
}
