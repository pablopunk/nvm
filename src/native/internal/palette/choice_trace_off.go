//go:build !automation

package palette

func (p *Palette) traceChoose(int, int) {}

func traceHoverChoose(int, pointerAt, pointerAt, float32, float32, float32, float32) {}

func traceArrow(string, int, int) {}

func (p *Palette) traceRows(bool, bool, int, int) {}
