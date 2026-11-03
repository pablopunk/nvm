package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

func itoa(i int) string { return strconv.Itoa(i) }

// ---- system commands ----

func run(name string, args ...string) {
	go func() {
		if err := exec.Command(name, args...).Run(); err != nil {
			fmt.Fprintln(os.Stderr, name, err)
		}
	}()
}

func (p *App) emptyTrash() {
	home, _ := os.UserHomeDir()
	for _, sub := range []string{"files", "info"} {
		dir := filepath.Join(home, ".local/share/Trash", sub)
		ents, _ := os.ReadDir(dir)
		for _, e := range ents {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// ---- timer ----

type Timer struct {
	End   time.Time
	Label string
	stop  chan struct{}
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
	if p.timer != nil {
		close(p.timer.stop)
	}
	t := &Timer{End: time.Now().Add(d), Label: d.String(), stop: make(chan struct{})}
	p.timer = t
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-t.stop:
				return
			case <-tick.C:
				left := time.Until(t.End)
				if left <= 0 {
					p.panel.Update(func() {
						p.timer = nil
						p.setTrayTitle("")
					})
					n := mygo.NewNotification(mygo.NotificationOptions{Title: "Timer finished", Body: t.Label + " is up"})
					_ = n.Show()
					return
				}
				txt := fmt.Sprintf("%d:%02d", int(left.Minutes()), int(left.Seconds())%60)
				p.panel.Update(func() { p.setTrayTitle(txt) })
			}
		}
	}()
}

func (p *App) cancelTimer() {
	if p.timer != nil {
		close(p.timer.stop)
		p.timer = nil
		p.setTrayTitle("")
	}
}
