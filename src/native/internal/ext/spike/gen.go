//go:build spike

// Package spike holds EXPERIMENTAL runtime measurements for WP3.1 (decision D1). It is a separate
// Go module on purpose: its third-party dependencies (yaegi, sobek) must never reach the main
// module, and nothing here is compiled without -tags spike. Results: docs/extension-runtime-spike.md.
package spike

import (
	"fmt"
	"strings"
)

// TrivialGo is the three-row extension used for load and per-call overhead.
const TrivialGo = `package main

import "nvm"

func Search(q string) []nvm.Item {
	return []nvm.Item{
		{Title: "one", Subtitle: "first", Value: "1"},
		{Title: "two", Subtitle: "second", Value: "2"},
		{Title: "three", Subtitle: "third", Value: "3"},
	}
}
`

// TrivialJS is the same three-row extension in JavaScript.
const TrivialJS = `function Search(q) {
  return [
    { title: "one", subtitle: "first", value: "1" },
    { title: "two", subtitle: "second", value: "2" },
    { title: "three", subtitle: "third", value: "3" },
  ];
}
`

const helperCount = 30

// RealisticGo returns a synthetic but shape-realistic extension of roughly 500 lines: a dataset
// built at init, thirty string helpers called through a table, JSON encoding, sorting and a
// Search that filters the dataset. It is not a real product extension; it is a size and mix probe.
func RealisticGo() string {
	var b strings.Builder
	b.WriteString(`package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"nvm"
)

type record struct {
	Name  string   ` + "`json:\"name\"`" + `
	Tags  []string ` + "`json:\"tags\"`" + `
	Score int      ` + "`json:\"score\"`" + `
}

var dataset []record

func init() {
	for i := 0; i < 200; i++ {
		tags := []string{"alpha", "beta", "gamma"}[:1+i%3]
		dataset = append(dataset, record{Name: fmt.Sprintf("item-%03d", i), Tags: tags, Score: (i * 37) % 101})
	}
}
`)
	for i := 0; i < helperCount; i++ {
		fmt.Fprintf(&b, `
func helper%02d(s string, n int) string {
	parts := strings.Split(s, "-")
	sort.Strings(parts)
	if n > len(parts) {
		n = len(parts)
	}
	out := strings.ToUpper(strings.Join(parts[:n], "_"))
	if len(out) > %d {
		out = out[:%d]
	}
	return fmt.Sprintf("%%s:%%d", out, len(parts)+%d)
}
`, i, 8+i%7, 8+i%7, i)
	}
	b.WriteString(`
var helpers = []func(string, int) string{`)
	for i := 0; i < helperCount; i++ {
		fmt.Fprintf(&b, "helper%02d, ", i)
	}
	b.WriteString(`}

func describe(r record) string {
	label := fmt.Sprintf("score %d · %s", r.Score, strings.Join(r.Tags, ","))
	for _, h := range helpers[:3] {
		label += " " + h(r.Name, 2)
	}
	return label
}

func encode(r record) string {
	data, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(data)
}

func matchTags(tags []string, q string) bool {
	for _, t := range tags {
		if strings.HasPrefix(t, q) {
			return true
		}
	}
	return false
}

func Search(q string) []nvm.Item {
	q = strings.ToLower(strings.TrimSpace(q))
	if len(q) < 2 {
		return nil
	}
	var out []nvm.Item
	for _, r := range dataset {
		if !strings.Contains(r.Name, q) && !matchTags(r.Tags, q) {
			continue
		}
		if len(out) >= 20 {
			break
		}
		out = append(out, nvm.Item{Title: r.Name, Subtitle: describe(r), Value: encode(r)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}
`)
	return b.String()
}

// RealisticJS is RealisticGo translated to JavaScript, with the same dataset, helpers and filter.
func RealisticJS() string {
	var b strings.Builder
	b.WriteString(`const dataset = [];
for (let i = 0; i < 200; i++) {
  const tags = ["alpha", "beta", "gamma"].slice(0, 1 + (i % 3));
  dataset.push({ name: "item-" + String(i).padStart(3, "0"), tags, score: (i * 37) % 101 });
}
`)
	for i := 0; i < helperCount; i++ {
		fmt.Fprintf(&b, `
function helper%02d(s, n) {
  const parts = s.split("-").sort();
  if (n > parts.length) n = parts.length;
  let out = parts.slice(0, n).join("_").toUpperCase();
  if (out.length > %d) out = out.slice(0, %d);
  return out + ":" + (parts.length + %d);
}
`, i, 8+i%7, 8+i%7, i)
	}
	b.WriteString(`
const helpers = [`)
	for i := 0; i < helperCount; i++ {
		fmt.Fprintf(&b, "helper%02d, ", i)
	}
	b.WriteString(`];

function describe(r) {
  let label = "score " + r.score + " · " + r.tags.join(",");
  for (const h of helpers.slice(0, 3)) label += " " + h(r.name, 2);
  return label;
}

function encode(r) {
  return JSON.stringify(r);
}

function matchTags(tags, q) {
  return tags.some((t) => t.startsWith(q));
}

function Search(q) {
  q = q.trim().toLowerCase();
  if (q.length < 2) return [];
  const out = [];
  for (const r of dataset) {
    if (!r.name.includes(q) && !matchTags(r.tags, q)) continue;
    if (out.length >= 20) break;
    out.push({ title: r.name, subtitle: describe(r), value: encode(r) });
  }
  out.sort((a, b) => (a.title < b.title ? -1 : a.title > b.title ? 1 : 0));
  return out;
}
`)
	return b.String()
}

// GenericsPrograms are five small generic programs, each defining Run() and returning a string
// that the test compares with the expected output.
var GenericsPrograms = []struct {
	Name string
	Src  string
	Want string
}{
	{"generic-func", `package main

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Run() string {
	return fmtInts(Map([]int{1, 2, 3}, func(i int) int { return i * 10 }))
}

func fmtInts(xs []int) string {
	s := ""
	for _, x := range xs {
		s += string(rune('0' + x/10))
	}
	return s
}
`, "123"},
	{"generic-type", `package main

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

func Run() string {
	s := &Stack[string]{}
	s.Push("a")
	s.Push("b")
	v, _ := s.Pop()
	return v
}
`, "b"},
	{"union-constraint", `package main

type Number interface {
	~int | ~float64
}

func Sum[T Number](xs []T) T {
	var total T
	for _, x := range xs {
		total += x
	}
	return total
}

func Run() string {
	if Sum([]float64{1.5, 2.5}) == 4 {
		return "4"
	}
	return "wrong"
}
`, "4"},
	{"stdlib-generic-call", `package main

import (
	"cmp"
	"slices"
)

func Run() string {
	xs := []string{"b", "c", "a"}
	slices.SortFunc(xs, func(a, b string) int { return cmp.Compare(a, b) })
	return xs[0] + xs[1] + xs[2]
}
`, "abc"},
	{"generic-pair-explicit", `package main

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

func MakePair[K comparable, V any](k K, v V) Pair[K, V] { return Pair[K, V]{Key: k, Val: v} }

func Run() string {
	p := MakePair[string, int]("answer", 42)
	if p.Val == 42 {
		return p.Key
	}
	return "wrong"
}
`, "answer"},
}
