package app

import (
	"fmt"
	"github.com/pablopunk/nvm/native/internal/search"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pablopunk/nvm/native/internal/palette"
)

func itoa(i int) string { return strconv.Itoa(i) }

// ---- processes ("Quit Apps" on Linux: user processes by memory) ----

type proc struct {
	PID  int
	Name string
	RSS  int // KB
}

func listProcs() []proc {
	uid := os.Getuid()
	ents, _ := os.ReadDir("/proc")
	agg := map[string]*proc{}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		var st syscall.Stat_t
		if syscall.Stat("/proc/"+e.Name(), &st) != nil || int(st.Uid) != uid {
			continue
		}
		comm, _ := os.ReadFile("/proc/" + e.Name() + "/comm")
		statm, _ := os.ReadFile("/proc/" + e.Name() + "/statm")
		f := strings.Fields(string(statm))
		if len(f) < 2 {
			continue
		}
		pages, _ := strconv.Atoi(f[1])
		name := strings.TrimSpace(string(comm))
		if a, ok := agg[name]; ok {
			a.RSS += pages * 4
		} else {
			agg[name] = &proc{PID: pid, Name: name, RSS: pages * 4}
		}
	}
	var out []proc
	for _, v := range agg {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RSS > out[j].RSS })
	return out
}

func (p *App) processesView() *palette.View {
	return &palette.View{
		Title: "Quit Applications", Placeholder: "Search Running Processes",
		Search: func(q string) []search.Item {
			lq := strings.ToLower(q)
			var out []search.Item
			for _, pr := range listProcs() {
				if lq != "" && !strings.Contains(strings.ToLower(pr.Name), lq) {
					continue
				}
				pr := pr
				kill := func(sig syscall.Signal) func(h search.Host) {
					return func(h search.Host) {
						// all processes of that name owned by the user
						n := 0
						ents, _ := os.ReadDir("/proc")
						for _, e := range ents {
							pid, err := strconv.Atoi(e.Name())
							if err != nil {
								continue
							}
							comm, _ := os.ReadFile("/proc/" + e.Name() + "/comm")
							if strings.TrimSpace(string(comm)) == pr.Name && pid != os.Getpid() && syscall.Kill(pid, sig) == nil {
								n++
							}
						}
						h.Toast(fmt.Sprintf("Signalled %d × %s", n, pr.Name))
						h.Refresh()
					}
				}
				out = append(out, search.Item{
					ID: pr.Name, Title: pr.Name, Subtitle: fmt.Sprintf("%.0f MB resident", float64(pr.RSS)/1024), Icon: "activity", Tone: "red",
					Actions: []search.Action{
						{Title: "Quit (SIGTERM)", Dismiss: search.DismissNever, Run: kill(syscall.SIGTERM)},
						{Title: "Force Quit (SIGKILL)", Label: "⌃⇧⌫", Mods: mod | ui.Shift, Key: ui.KeyBackspace, Dismiss: search.DismissNever, Run: kill(syscall.SIGKILL)},
					},
				})
				if len(out) >= 60 {
					break
				}
			}
			return out
		},
	}
}

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
