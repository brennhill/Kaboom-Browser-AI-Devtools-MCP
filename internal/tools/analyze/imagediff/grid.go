// Purpose: Builds a per-pixel boolean grid marking changed regions between baseline and current images.
// Why: Separates grid construction from region detection, rendering, and I/O.
// absDiff16 lives here rather than in a general "math" file: RebuildChangedGrid is
// its only caller, and the 257x threshold scaling below is meaningless without it.
package imagediff

import "image"

func RebuildChangedGrid(baseline, current image.Image, threshold int) [][]bool {
	bBounds := baseline.Bounds()
	cBounds := current.Bounds()
	bW, bH := bBounds.Dx(), bBounds.Dy()
	cW, cH := cBounds.Dx(), cBounds.Dy()

	intW := min(bW, cW)
	intH := min(bH, cH)
	maxW := max(bW, cW)
	maxH := max(bH, cH)

	changed := make([][]bool, maxH)
	for y := range changed {
		changed[y] = make([]bool, maxW)
	}

	for y := 0; y < intH; y++ {
		for x := 0; x < intW; x++ {
			r1, g1, b1, a1 := baseline.At(bBounds.Min.X+x, bBounds.Min.Y+y).RGBA()
			r2, g2, b2, a2 := current.At(cBounds.Min.X+x, cBounds.Min.Y+y).RGBA()
			delta := absDiff16(r1, r2) + absDiff16(g1, g2) + absDiff16(b1, b2) + absDiff16(a1, a2)
			if delta > uint32(threshold)*257 {
				changed[y][x] = true
			}
		}
	}

	if bW != cW || bH != cH {
		for y := 0; y < maxH; y++ {
			for x := 0; x < maxW; x++ {
				if x >= intW || y >= intH {
					changed[y][x] = true
				}
			}
		}
	}

	return changed
}

// countChanged returns the number of true cells in a changed grid.
func countChanged(changed [][]bool) int {
	n := 0
	for _, row := range changed {
		for _, c := range row {
			if c {
				n++
			}
		}
	}
	return n
}

func absDiff16(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
