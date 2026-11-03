//go:build rig

package rigtest

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	paletteTitle  = "Nevermind"
	pollInterval  = 50 * time.Millisecond
	idleTimeoutMS = 2000
	callTimeout   = 10 * time.Second
)

// automationClient speaks the MyGo automation protocol (docs/plugins/automation.md): one JSON
// object per line on a Unix socket, answered in order.
type automationClient struct {
	conn   net.Conn
	reader *bufio.Reader
	nextID int
}

// AutomationError is an error answer of the app, with the code the protocol defines.
type AutomationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AutomationError) Error() string { return e.Code + ": " + e.Message }

// Rect is a frame in DIPs relative to the window's content, as the protocol reports it.
type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Window is one entry of the `windows` answer.
type Window struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Focused bool   `json:"focused"`
	Visible bool   `json:"visible"`
	Bounds  Rect   `json:"bounds"`
}

// Node is an element of a window's last frame, the parts of it the journeys match on.
type Node struct {
	ID          string   `json:"id"`
	Role        string   `json:"role"`
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Value       string   `json:"value"`
	Placeholder string   `json:"placeholder"`
	Description string   `json:"description"`
	States      []string `json:"states"`
}

// Tree is the `tree` answer: the elements of one window.
type Tree struct {
	Window int    `json:"window"`
	Focus  string `json:"focus"`
	Nodes  []Node `json:"nodes"`
}

func dialAutomation(socket string, timeout time.Duration) (*automationClient, error) {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("unix", socket, time.Second)
		if err == nil {
			return &automationClient{conn: conn, reader: bufio.NewReader(conn)}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("dial %s: %w", socket, err)
		}
		time.Sleep(pollInterval)
	}
}

func (c *automationClient) call(method string, params any) (json.RawMessage, error) {
	c.nextID++
	request, err := json.Marshal(map[string]any{"id": c.nextID, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if err := c.conn.SetDeadline(time.Now().Add(callTimeout)); err != nil {
		return nil, err
	}
	if _, err := c.conn.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("automation %s: %w", method, err)
	}
	answer, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("automation %s: %w", method, err)
	}
	var reply struct {
		Result json.RawMessage  `json:"result"`
		Error  *AutomationError `json:"error"`
	}
	if err := json.Unmarshal(answer, &reply); err != nil {
		return nil, fmt.Errorf("automation %s: bad answer %q: %w", method, answer, err)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("automation %s: %w", method, reply.Error)
	}
	return reply.Result, nil
}

// Windows lists the app's windows, hidden ones included.
func (a *App) Windows() ([]Window, error) {
	raw, err := a.client.call("windows", nil)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Windows []Window `json:"windows"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, err
	}
	return reply.Windows, nil
}

// PaletteWindow is the app's palette window: the one of native UI titled like the app.
func (a *App) PaletteWindow() (Window, error) {
	windows, err := a.Windows()
	if err != nil {
		return Window{}, err
	}
	for _, w := range windows {
		if w.Title == paletteTitle && w.Kind == "content" {
			return w, nil
		}
	}
	return Window{}, fmt.Errorf("no palette window among %+v", windows)
}

// WaitPalette polls the palette window until ready reports true, for up to timeout.
func (a *App) WaitPalette(timeout time.Duration, what string, ready func(Window) bool) (Window, error) {
	deadline := time.Now().Add(timeout)
	for {
		w, err := a.PaletteWindow()
		if err == nil && ready(w) {
			return w, nil
		}
		if time.Now().After(deadline) {
			return w, fmt.Errorf("palette did not become %s within %s (last: %+v, err: %v)", what, timeout, w, err)
		}
		time.Sleep(pollInterval)
	}
}

// Tree waits for the palette to settle (`idle`) and returns its elements.
func (a *App) Tree() (Tree, error) {
	w, err := a.PaletteWindow()
	if err != nil {
		return Tree{}, err
	}
	if _, err := a.client.call("idle", map[string]any{"window": w.ID, "timeout": idleTimeoutMS}); err != nil && !isCode(err, "timeout") {
		return Tree{}, err
	}
	raw, err := a.client.call("tree", map[string]any{"window": w.ID})
	if err != nil {
		return Tree{}, err
	}
	var tree Tree
	if err := json.Unmarshal(raw, &tree); err != nil {
		return Tree{}, err
	}
	return tree, nil
}

func isCode(err error, code string) bool {
	var automation *AutomationError
	return errors.As(err, &automation) && automation.Code == code
}

// WaitFor polls the palette's elements until one has text in its label, value, placeholder or
// description. It fails with the labels it saw, never with a screenshot comparison: the journeys
// assert on what assistive technology reads.
func (a *App) WaitFor(text string, timeout time.Duration) (Node, error) {
	deadline := time.Now().Add(timeout)
	var seen []string
	for {
		tree, err := a.Tree()
		if err != nil {
			return Node{}, err
		}
		seen = seen[:0]
		for _, node := range tree.Nodes {
			if node.Label != "" {
				seen = append(seen, node.Label)
			}
			if nodeHasText(node, text) {
				return node, nil
			}
		}
		if time.Now().After(deadline) {
			return Node{}, fmt.Errorf("no element with text %q within %s; labels seen: %s", text, timeout, summarize(seen))
		}
		time.Sleep(pollInterval)
	}
}

// FindNode returns the first element of the palette matching role and label, without waiting.
func (a *App) FindNode(role, label string) (Node, error) {
	tree, err := a.Tree()
	if err != nil {
		return Node{}, err
	}
	for _, node := range tree.Nodes {
		if node.Role == role && node.Label == label {
			return node, nil
		}
	}
	return Node{}, fmt.Errorf("no %s labelled %q", role, label)
}

func nodeHasText(node Node, text string) bool {
	return strings.Contains(node.Label, text) || strings.Contains(node.Value, text) ||
		strings.Contains(node.Placeholder, text) || strings.Contains(node.Description, text)
}

func summarize(labels []string) string {
	const maxLabels = 25
	if len(labels) > maxLabels {
		labels = labels[:maxLabels]
	}
	return fmt.Sprintf("%q", labels)
}

// CapturePalette saves the palette window as rendered (not the desktop) to the test's artifacts.
func (a *App) CapturePalette(name string) (string, error) {
	w, err := a.PaletteWindow()
	if err != nil {
		return "", err
	}
	raw, err := a.client.call("capture", map[string]any{"window": w.ID})
	if err != nil {
		return "", err
	}
	var reply struct {
		PNG string `json:"png"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(reply.PNG)
	if err != nil {
		return "", fmt.Errorf("capture %s: %w", name, err)
	}
	path := a.artifactPath(name + "-palette.png")
	return path, os.WriteFile(path, data, 0o644)
}
