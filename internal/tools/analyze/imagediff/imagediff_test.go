// Purpose: Tests for image pixel-diff computation.
// Docs: docs/features/feature/analyze-tool/index.md

// imagediff_test.go — Tests for pure Go pixel-diff comparison.
package imagediff

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// createTestImage generates a solid-color PNG at the given path.
func createTestImage(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
}

// createTestImageWithPixel generates a solid-color PNG with one changed pixel.
func createTestImageWithPixel(t *testing.T, path string, w, h int, bg, pixel color.Color, px, py int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	img.Set(px, py, pixel)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test image: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test image: %v", err)
	}
}

func TestCompareImages_IdenticalImages(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	createTestImage(t, a, 100, 100, color.White)
	createTestImage(t, b, 100, 100, color.White)

	result, err := CompareImages(a, b, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DiffPercentage != 0 {
		t.Errorf("expected 0%% diff, got %.2f%%", result.DiffPercentage)
	}
	if result.PixelsChanged != 0 {
		t.Errorf("expected 0 pixels changed, got %d", result.PixelsChanged)
	}
	if !result.DimensionsMatch {
		t.Error("expected dimensions to match")
	}
	if result.Verdict != "identical" {
		t.Errorf("expected verdict 'identical', got %q", result.Verdict)
	}
}

func TestCompareImages_SmallDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	createTestImage(t, a, 10, 10, color.White)
	// b has one red pixel at (5,5) — definitely above threshold
	createTestImageWithPixel(t, b, 10, 10, color.White, color.RGBA{255, 0, 0, 255}, 5, 5)

	result, err := CompareImages(a, b, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PixelsChanged != 1 {
		t.Errorf("expected 1 pixel changed, got %d", result.PixelsChanged)
	}
	if result.PixelsTotal != 100 {
		t.Errorf("expected 100 total pixels, got %d", result.PixelsTotal)
	}
	if len(result.Regions) == 0 {
		t.Error("expected at least 1 region")
	}
}

func TestCompareImages_DimensionMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	createTestImage(t, a, 100, 100, color.White)
	createTestImage(t, b, 120, 100, color.White)

	result, err := CompareImages(a, b, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DimensionsMatch {
		t.Error("expected dimensions_match to be false")
	}
	// Pixels outside the intersection (20x100=2000) should be marked as changed
	if result.PixelsChanged < 2000 {
		t.Errorf("expected at least 2000 changed pixels from dimension mismatch, got %d", result.PixelsChanged)
	}
}

func TestCompareImages_ThresholdFiltering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	createTestImage(t, a, 10, 10, color.RGBA{100, 100, 100, 255})
	// Pixel at (5,5) is just barely different — delta = 3 (below threshold of 30)
	createTestImageWithPixel(t, b, 10, 10, color.RGBA{100, 100, 100, 255}, color.RGBA{101, 101, 101, 255}, 5, 5)

	result, err := CompareImages(a, b, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PixelsChanged != 0 {
		t.Errorf("expected 0 pixels changed (below threshold), got %d", result.PixelsChanged)
	}
	if result.Verdict != "identical" {
		t.Errorf("expected verdict 'identical', got %q", result.Verdict)
	}
}

func TestWriteDiffImage_OutputValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	baseline := image.NewRGBA(image.Rect(0, 0, 10, 10))
	current := image.NewRGBA(image.Rect(0, 0, 10, 10))
	changed := make([][]bool, 10)
	for y := range changed {
		changed[y] = make([]bool, 10)
	}
	changed[5][5] = true

	outPath := filepath.Join(dir, "diff.png")
	err := WriteDiffImage(baseline, current, changed, outPath)
	if err != nil {
		t.Fatalf("writeDiffImage error: %v", err)
	}

	// Verify the output is a valid PNG
	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("open diff image: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode diff image: %v", err)
	}
	if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 10 {
		t.Errorf("expected 10x10, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestFindChangedRegions_GroupsAdjacent(t *testing.T) {
	t.Parallel()
	// 5x5 grid with a 2x2 block of changed pixels at (1,1)
	changed := make([][]bool, 5)
	for y := range changed {
		changed[y] = make([]bool, 5)
	}
	changed[1][1] = true
	changed[1][2] = true
	changed[2][1] = true
	changed[2][2] = true

	regions := findChangedRegions(changed, 1)
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	r := regions[0]
	if r.X != 1 || r.Y != 1 || r.Width != 2 || r.Height != 2 {
		t.Errorf("expected region {1,1,2,2}, got {%d,%d,%d,%d}", r.X, r.Y, r.Width, r.Height)
	}
}

func TestFindChangedRegions_MultipleDisjoint(t *testing.T) {
	t.Parallel()
	// 10x10 grid with two separate single-pixel regions
	changed := make([][]bool, 10)
	for y := range changed {
		changed[y] = make([]bool, 10)
	}
	changed[0][0] = true
	changed[9][9] = true

	regions := findChangedRegions(changed, 1)
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
}

// Moved here from analyze/visual_diff_test.go: it exercises DiffVerdict, which is
// image-diff code, not visual-baseline argument parsing. Leaving it behind would
// have left DiffVerdict reading 0.0% in this package's coverage profile.
func TestVisualDiffVerdict(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pct     float64
		verdict string
	}{
		{0, "identical"},
		{0.5, "minor_changes"},
		{4.9, "minor_changes"},
		{5.0, "major_changes"},
		{24.9, "major_changes"},
		{25.0, "completely_different"},
		{100, "completely_different"},
	}
	for _, tc := range tests {
		v := DiffVerdict(tc.pct)
		if v != tc.verdict {
			t.Errorf("DiffVerdict(%.1f) = %q, want %q", tc.pct, v, tc.verdict)
		}
	}
}
