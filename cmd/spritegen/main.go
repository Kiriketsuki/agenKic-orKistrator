// Command spritegen generates the pixel-art placeholder sprites for the
// Godot UI: one 16x20 wizard per character class and a 32x32 tileable
// stone-brick floor texture. Output goes to godot/assets/sprites/ and
// godot/assets/tiles/. Demo/dev use only — rerun after tweaking palettes.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

// 16x20 wizard. Legend: . transparent, H hat, B hat band, S skin, E eye,
// R robe, D robe shade, W staff wood, O staff orb.
var wizard = []string{
	"................",
	".......HH.......",
	"......HHHH......",
	".....HHHHHH.....",
	"....HHHHHHHH....",
	"...HHHHHHHHHH...",
	"..BBBBBBBBBBBB..",
	".....SSSSSS.....",
	"....SSESSES...O.",
	"....SSSSSSS...W.",
	".....SSSS.....W.",
	"...RRRRRRRR...W.",
	"..RRRRRRRRRR..W.",
	"..RRDRRRRDRR..W.",
	"..RRDRRRRDRR..W.",
	".RRRRRRRRRRRR.W.",
	".RRRRRRRRRRRR.W.",
	".RRRRRRRRRRRR.W.",
	".DDRRRRRRRRDD...",
	".DDDDDDDDDDDD...",
}

type palette struct {
	robe  color.NRGBA
	shade color.NRGBA
	hat   color.NRGBA
	band  color.NRGBA
	orb   color.NRGBA
}

func shadeOf(c color.NRGBA) color.NRGBA {
	return color.NRGBA{R: c.R / 2, G: c.G / 2, B: c.B / 2, A: 255}
}

func mkPalette(robe, orb color.NRGBA) palette {
	return palette{
		robe:  robe,
		shade: shadeOf(robe),
		hat:   color.NRGBA{R: robe.R/2 + 40, G: robe.G/2 + 40, B: robe.B/2 + 40, A: 255},
		band:  color.NRGBA{R: 240, G: 210, B: 120, A: 255},
		orb:   orb,
	}
}

// Mirrors CLASS_COLORS in agent_character.gd.
var classes = map[string]palette{
	"alchemist":  mkPalette(color.NRGBA{217, 140, 0, 255}, color.NRGBA{120, 255, 120, 255}),
	"scribe":     mkPalette(color.NRGBA{89, 153, 242, 255}, color.NRGBA{255, 255, 180, 255}),
	"archmage":   mkPalette(color.NRGBA{191, 51, 230, 255}, color.NRGBA{140, 240, 255, 255}),
	"wardkeeper": mkPalette(color.NRGBA{64, 191, 77, 255}, color.NRGBA{255, 220, 120, 255}),
	"librarian":  mkPalette(color.NRGBA{166, 102, 38, 255}, color.NRGBA{200, 240, 255, 255}),
	"enchanter":  mkPalette(color.NRGBA{26, 217, 204, 255}, color.NRGBA{255, 160, 220, 255}),
	"apprentice": mkPalette(color.NRGBA{166, 166, 191, 255}, color.NRGBA{200, 200, 255, 255}),
}

var (
	skin  = color.NRGBA{240, 200, 160, 255}
	eye   = color.NRGBA{40, 40, 60, 255}
	wood  = color.NRGBA{120, 80, 40, 255}
	clear = color.NRGBA{0, 0, 0, 0}
)

func drawWizard(p palette) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 20))
	for y, row := range wizard {
		for x, ch := range row {
			var c color.NRGBA
			switch ch {
			case 'H':
				c = p.hat
			case 'B':
				c = p.band
			case 'S':
				c = skin
			case 'E':
				c = eye
			case 'R':
				c = p.robe
			case 'D':
				c = p.shade
			case 'W':
				c = wood
			case 'O':
				c = p.orb
			default:
				c = clear
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// drawStoneTile builds a 32x32 tileable brick pattern with per-brick shade
// variation, highlighted top edges, and occasional moss.
func drawStoneTile() *image.NRGBA {
	mortar := color.NRGBA{30, 32, 38, 255}
	moss := color.NRGBA{72, 96, 60, 255}
	shades := []color.NRGBA{
		{78, 82, 96, 255},
		{70, 74, 88, 255},
		{86, 90, 104, 255},
		{64, 68, 80, 255},
	}

	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		row := y / 8
		offset := (row % 2) * 8
		for x := 0; x < 32; x++ {
			brickIdx := ((x + offset) / 16) + row*3
			c := shades[brickIdx%len(shades)]
			switch {
			case y%8 == 7 || (x+offset)%16 == 15:
				c = mortar
			case y%8 == 0: // top-edge highlight per brick
				c = color.NRGBA{c.R + 18, c.G + 18, c.B + 18, 255}
			case (x*7+y*13)%37 == 0:
				c = moss
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// drawSky builds a 256x512 vertical night gradient with stars and a moon.
func drawSky() *image.NRGBA {
	top := [3]float64{14, 12, 34}
	bottom := [3]float64{58, 44, 82}
	img := image.NewNRGBA(image.Rect(0, 0, 256, 512))
	for y := 0; y < 512; y++ {
		t := float64(y) / 511.0
		c := color.NRGBA{
			R: uint8(top[0] + (bottom[0]-top[0])*t),
			G: uint8(top[1] + (bottom[1]-top[1])*t),
			B: uint8(top[2] + (bottom[2]-top[2])*t),
			A: 255,
		}
		for x := 0; x < 256; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	// Deterministic star field, denser near the top.
	for i := 0; i < 180; i++ {
		x := (i*97 + 31) % 256
		y := ((i*53+17)*(i%7+1) + i*i*3) % 512
		if y > 380 {
			continue
		}
		b := uint8(150 + (i*37)%105)
		img.SetNRGBA(x, y, color.NRGBA{b, b, uint8(min(255, int(b)+20)), 255})
		if i%9 == 0 { // a few bigger twinkles
			img.SetNRGBA((x+1)%256, y, color.NRGBA{b, b, 255, 200})
			if y+1 < 512 {
				img.SetNRGBA(x, y+1, color.NRGBA{b, b, 255, 200})
			}
		}
	}
	// Moon with a crater, top-right.
	moonC := color.NRGBA{226, 224, 210, 255}
	for dy := -9; dy <= 9; dy++ {
		for dx := -9; dx <= 9; dx++ {
			if dx*dx+dy*dy <= 81 {
				img.SetNRGBA(206+dx, 56+dy, moonC)
			}
		}
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx*dx+dy*dy <= 4 {
				img.SetNRGBA(203+dx, 54+dy, color.NRGBA{200, 198, 186, 255})
			}
		}
	}
	return img
}

// drawWindow builds a 10x14 arched window with a warm glow.
func drawWindow() *image.NRGBA {
	frame := color.NRGBA{34, 30, 42, 255}
	glowIn := color.NRGBA{255, 214, 120, 255}
	glowOut := color.NRGBA{214, 150, 70, 255}
	img := image.NewNRGBA(image.Rect(0, 0, 10, 14))
	for y := 0; y < 14; y++ {
		for x := 0; x < 10; x++ {
			// Arch: top two rows narrow.
			inArch := true
			switch y {
			case 0:
				inArch = x >= 3 && x <= 6
			case 1:
				inArch = x >= 2 && x <= 7
			default:
				inArch = x >= 1 && x <= 8
			}
			if !inArch {
				img.SetNRGBA(x, y, clear)
				continue
			}
			edge := x <= 1 || x >= 8 || y >= 13 || y <= 1 || x == 4 || x == 5 && false
			center := x >= 3 && x <= 6 && y >= 3 && y <= 10
			switch {
			case edge:
				img.SetNRGBA(x, y, frame)
			case center:
				img.SetNRGBA(x, y, glowIn)
			default:
				img.SetNRGBA(x, y, glowOut)
			}
		}
	}
	// Mullion cross.
	for y := 2; y < 13; y++ {
		img.SetNRGBA(5, y, frame)
	}
	for x := 1; x < 9; x++ {
		img.SetNRGBA(x, 7, frame)
	}
	return img
}

// drawTorch builds a 6x12 wall torch with flame.
func drawTorch() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 6, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 6; x++ {
			img.SetNRGBA(x, y, clear)
		}
	}
	flameOut := color.NRGBA{255, 140, 40, 255}
	flameIn := color.NRGBA{255, 230, 120, 255}
	// Flame.
	img.SetNRGBA(2, 0, flameOut)
	img.SetNRGBA(3, 0, flameOut)
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 4; x++ {
			img.SetNRGBA(x, y, flameOut)
		}
	}
	img.SetNRGBA(2, 2, flameIn)
	img.SetNRGBA(3, 2, flameIn)
	img.SetNRGBA(2, 3, flameIn)
	img.SetNRGBA(3, 3, flameIn)
	// Handle + bracket.
	for y := 4; y <= 10; y++ {
		img.SetNRGBA(2, y, wood)
		img.SetNRGBA(3, y, wood)
	}
	img.SetNRGBA(1, 10, color.NRGBA{90, 90, 100, 255})
	img.SetNRGBA(4, 10, color.NRGBA{90, 90, 100, 255})
	return img
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func save(path string, img image.Image) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatalf("encode %s: %v", path, err)
	}
	log.Printf("wrote %s", path)
}

func main() {
	for name, pal := range classes {
		save(filepath.Join("godot", "assets", "sprites", "agents", name+".png"), drawWizard(pal))
	}
	save(filepath.Join("godot", "assets", "tiles", "stone_floor.png"), drawStoneTile())
	save(filepath.Join("godot", "assets", "tiles", "sky.png"), drawSky())
	save(filepath.Join("godot", "assets", "sprites", "deco", "window.png"), drawWindow())
	save(filepath.Join("godot", "assets", "sprites", "deco", "torch.png"), drawTorch())
}
