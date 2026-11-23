// dimensions.go — Validates terminal dimensions before uint16 conversion.

package dimensions

// Resolve converts bounded terminal dimensions without integer truncation.
func Resolve(cols, rows int) (uint16, uint16, bool) {
	if cols < 0 || rows < 0 || cols > 65535 || rows > 65535 {
		return 0, 0, false
	}
	// #nosec G115 -- both values are explicitly bounded to uint16 above.
	return uint16(cols), uint16(rows), true
}
