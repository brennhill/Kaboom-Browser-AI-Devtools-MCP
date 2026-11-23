// builder.go — Provides small shared string-builder helpers for annotation artifact generation.
// Why: Avoids duplicated line/format append logic across report and visual-test emitters.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotations

import "fmt"

type builder struct {
	buf []byte
}

func (b *builder) line(s string) {
	b.buf = append(b.buf, s...)
	b.buf = append(b.buf, '\n')
}

func (b *builder) linef(format string, args ...any) {
	b.buf = append(b.buf, fmt.Sprintf(format, args...)...)
	b.buf = append(b.buf, '\n')
}

func (b *builder) string() string {
	return string(b.buf)
}
