//go:build !automation

package palette

func (p *Palette) traceChoose(int, int) {}

func traceHoverChoose(int, pointerAt, pointerAt, float32, float32, float32, float32) {}

func traceArrow(string, int, int) {}
