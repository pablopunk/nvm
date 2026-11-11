//go:build automation

package palette

import (
	"runtime"
	"strings"

	"github.com/pablopunk/nvm/native/internal/diag"
	"github.com/pablopunk/nvm/native/internal/logx"
)

// traceChoose records a choice change and the function that asked for it, in rig builds only.
func (p *Palette) traceChoose(request, before int) {
	cause := "unknown"
	if pc, _, _, ok := runtime.Caller(2); ok {
		cause = strings.TrimPrefix(runtime.FuncForPC(pc).Name(), "github.com/pablopunk/nvm/native/internal/palette.")
	}
	diag.ActionTrace("choose", logx.F{"cause": cause, "request": request, "rows": len(p.results), "before": before, "after": p.chosen})
}

// traceHoverChoose records a pointer-driven choice with the pointer positions that caused it.
func traceHoverChoose(row int, at, prev pointerAt, bx, by, lx, ly float32) {
	diag.ActionTrace("hover-choose", logx.F{
		"row": row, "x": at.x, "y": at.y, "prev_x": prev.x, "prev_y": prev.y,
		"bounds_x": bx, "bounds_y": by, "local_x": lx, "local_y": ly,
	})
}

// traceArrow records an arrow key applied to the choice.
func traceArrow(key string, before, after int) {
	diag.ActionTrace("arrow", logx.F{"key": key, "before": before, "after": after})
}

// traceRows records a ranking answer landing and what it did to the choice.
func (p *Palette) traceRows(newQuery, sameView bool, before, rows int) {
	diag.ActionTrace("show-rows", logx.F{"new_query": newQuery, "same_view": sameView, "pinned": p.choicePinned, "before": before, "after": p.chosen, "rows": rows})
}
