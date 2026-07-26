// Purpose: Detects contiguous changed regions in the pixel diff grid via flood-fill.
// Why: Separates region detection from grid construction, rendering, and I/O.
package imagediff

func findChangedRegions(changed [][]bool, minSize int) []Region {
	h := len(changed)
	if h == 0 {
		return nil
	}
	w := len(changed[0])

	visited := make([][]bool, h)
	for y := range visited {
		visited[y] = make([]bool, w)
	}

	var regions []Region
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !changed[y][x] || visited[y][x] {
				continue
			}

			minX, minY, maxX, maxY := x, y, x, y
			queue := [][2]int{{x, y}}
			visited[y][x] = true
			count := 0

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				cx, cy := cur[0], cur[1]
				count++

				if cx < minX {
					minX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cx > maxX {
					maxX = cx
				}
				if cy > maxY {
					maxY = cy
				}

				for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h && changed[ny][nx] && !visited[ny][nx] {
						visited[ny][nx] = true
						queue = append(queue, [2]int{nx, ny})
					}
				}
			}

			if count >= minSize {
				regions = append(regions, Region{
					X:      minX,
					Y:      minY,
					Width:  maxX - minX + 1,
					Height: maxY - minY + 1,
				})
			}
		}
	}

	return regions
}
