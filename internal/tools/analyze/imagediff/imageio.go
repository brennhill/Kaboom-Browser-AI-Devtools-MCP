// Purpose: Reads baseline/current screenshots from disk and writes the rendered diff PNG.
// Why: Concentrates every filesystem touch in the package into one file, so the diff
// computation in imagediff.go, grid.go and regions.go stays pure and testable in memory.
package imagediff

import (
	"image"
	"image/color"
	_ "image/jpeg" // Register JPEG decoder.
	"image/png"
	"os"
)

// LoadImage decodes a PNG or JPEG image from the given file path.
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// WriteDiffImage renders a side-by-side diff image highlighting changed pixels and saves it to path.
func WriteDiffImage(baseline, current image.Image, changed [][]bool, path string) error {
	bBounds := baseline.Bounds()
	h := len(changed)
	w := 0
	if h > 0 {
		w = len(changed[0])
	}

	diff := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if changed[y][x] {
				diff.Set(x, y, color.RGBA{255, 0, 255, 255})
				continue
			}

			bx := bBounds.Min.X + x
			by := bBounds.Min.Y + y
			if bx < bBounds.Max.X && by < bBounds.Max.Y {
				r, g, b, _ := baseline.At(bx, by).RGBA()
				diff.Set(x, y, color.RGBA{
					dimmedChannel(r),
					dimmedChannel(g),
					dimmedChannel(b),
					255,
				})
			} else {
				diff.Set(x, y, color.RGBA{20, 20, 20, 255})
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, diff)
}

func dimmedChannel(channel uint32) uint8 {
	// RGBA returns a 16-bit channel in uint32; shifting produces [0,255], and
	// multiplying by 77/255 can only reduce that bound.
	return uint8(channel>>8) * 77 / 255 // #nosec G115 -- channel>>8 is bounded to uint8.
}
