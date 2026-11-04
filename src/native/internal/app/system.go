package app

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/menubar"
)

func itoa(i int) string { return strconv.Itoa(i) }

// ---- timer ----

// timerJob names the timer's render on the menu bar scheduler. A timer replaces its job; cancelling removes it.
const timerJob = "timer"

type Timer struct {
	End   time.Time
	Label string
}

var durRe = regexp.MustCompile(`^(?:timer\s+|countdown\s+)?(\d+(?:\.\d+)?)\s*(s|sec|m|min|minutes?|h|hr|hours?)?$`)

func parseTimer(q string) (time.Duration, bool) {
	q = strings.ToLower(strings.TrimSpace(q))
	if !strings.HasPrefix(q, "timer") && !strings.HasPrefix(q, "countdown") {
		return 0, false
	}
	m := durRe.FindStringSubmatch(q)
	if m == nil {
		return 0, false
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	switch {
	case strings.HasPrefix(m[2], "s"):
		return time.Duration(v * float64(time.Second)), true
	case strings.HasPrefix(m[2], "h"):
		return time.Duration(v * float64(time.Hour)), true
	}
	return time.Duration(v * float64(time.Minute)), true
}

func (p *App) startTimer(d time.Duration) {
	t := &Timer{End: time.Now().Add(d), Label: d.String()}
	p.timer = t
	p.menubarJobs.Schedule(timerJob, menubar.Job{Every: time.Second, First: time.Second, Run: p.timerRender(t)})
}

// timerRender shows the time left in the menu bar title each second. When the time is up it clears the title and
// notifies, and it stops its own job.
func (p *App) timerRender(t *Timer) menubar.Render {
	return func(context.Context) func() {
		left := time.Until(t.End)
		if left <= 0 {
			p.menubarJobs.Cancel(timerJob)
			n := mygo.NewNotification(mygo.NotificationOptions{Title: "Timer finished", Body: t.Label + " is up"})
			_ = n.Show()
			return func() {
				if p.timer == t {
					p.timer = nil
					p.setTrayTitle("")
				}
			}
		}
		txt := fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
		return func() {
			if p.timer == t {
				p.setTrayTitle(txt)
			}
		}
	}
}

func (p *App) cancelTimer() {
	if p.timer != nil {
		p.menubarJobs.Cancel(timerJob)
		p.timer = nil
		p.setTrayTitle("")
	}
}
