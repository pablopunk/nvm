package geometry

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// Property tests for display moves, neighbor search and outcome verification.

func TestPropertyAdjacentDisplayIsARingWithInverseSteps(t *testing.T) {
	rng := newPropertyRand(7)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		count := 2 + rng.IntN(4)
		displays := make([]Display, 0, count)
		for index := range count {
			displays = append(displays, randomDisplay(rng, fmt.Sprintf("d%d", index)))
		}
		order := []DisplayOrder{DisplayOrderLeftToRight, DisplayOrderTopToBottom}[rng.IntN(2)]
		start := displays[rng.IntN(count)]
		current := start
		for step := 0; step < count; step++ {
			next, ok := AdjacentDisplay(displays, current, DirectionNext, order)
			if !ok {
				t.Fatalf("iteration %d: no next display from %s", iteration, current.ID)
			}
			back, ok := AdjacentDisplay(displays, next, DirectionPrevious, order)
			if !ok || back.ID != current.ID {
				t.Fatalf("iteration %d: previous from %s gave %s, want %s", iteration, next.ID, back.ID, current.ID)
			}
			current = next
		}
		if current.ID != start.ID {
			t.Fatalf("iteration %d: %d next steps ended on %s, want %s", iteration, count, current.ID, start.ID)
		}
	}
}

func TestPropertyDisplayMoveFitsTheTargetWorkArea(t *testing.T) {
	rng := newPropertyRand(8)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		displays := []Display{randomDisplay(rng, "a"), randomDisplay(rng, "b")}
		displays[0].Main = true
		window := randomWindow(rng, displays[0])
		window.IsResizable = true
		options := randomGaps(rng)
		options.DisplayMoveSizing = []DisplayMoveSizing{MoveProportional, MoveKeepSize}[rng.IntN(2)]
		frame, target, err := MoveWindowToAdjacentDisplay(displays, window, DirectionNext, options)
		if err != nil {
			t.Fatalf("iteration %d: %v", iteration, err)
		}
		assertPropertyFrame(t, iteration, "display move", frame, target.WorkArea, window)
	}
}

// randomNeighborField returns unique-ID candidates for a neighbor search near one origin.
func randomNeighborField(rng *rand.Rand, count int) (NeighborWindow, []NeighborWindow) {
	displayIDs := []string{"a", "b"}
	makeWindow := func(id string) NeighborWindow {
		return NeighborWindow{
			ID:           id,
			Frame:        Rect{X: float64(rng.IntN(1200) - 100), Y: float64(rng.IntN(900) - 100), Width: float64(rng.IntN(500) - 20), Height: float64(rng.IntN(500) - 20)},
			ZIndex:       rng.IntN(4),
			DisplayID:    displayIDs[rng.IntN(2)],
			IsFullScreen: rng.IntN(10) == 0,
		}
	}
	origin := makeWindow("origin")
	candidates := make([]NeighborWindow, 0, count)
	for index := range count {
		candidates = append(candidates, makeWindow(fmt.Sprintf("w%02d", index)))
	}
	return origin, candidates
}

var neighborDirections = []NeighborDirection{NeighborLeft, NeighborRight, NeighborUp, NeighborDown}

func TestPropertyNeighborIsACandidateAndIgnoresTheCandidateOrder(t *testing.T) {
	rng := newPropertyRand(9)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		origin, candidates := randomNeighborField(rng, rng.IntN(9))
		direction := neighborDirections[rng.IntN(4)]
		options := NeighborOptions{Wrap: rng.IntN(2) == 0}
		id, found := NearestWindowInDirection(origin, candidates, direction, options)
		if found {
			chosen, ok := findNeighbor(candidates, id)
			if !ok {
				t.Fatalf("iteration %d: returned %q which is not a candidate", iteration, id)
			}
			if !chosen.Frame.HasArea() || (origin.IsFullScreen && chosen.DisplayID == origin.DisplayID) {
				t.Fatalf("iteration %d: returned an ineligible window %+v", iteration, chosen)
			}
		}
		shuffled := slices.Clone(candidates)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		shuffledID, shuffledFound := NearestWindowInDirection(origin, shuffled, direction, options)
		if shuffledID != id || shuffledFound != found {
			t.Fatalf("iteration %d: candidate order changed the answer: (%q, %v) vs (%q, %v)", iteration, id, found, shuffledID, shuffledFound)
		}
	}
}

func findNeighbor(candidates []NeighborWindow, id string) (NeighborWindow, bool) {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return NeighborWindow{}, false
}

func TestPropertyWrapOnlyAddsAnswersWhenNothingLiesAhead(t *testing.T) {
	rng := newPropertyRand(10)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		origin, candidates := randomNeighborField(rng, rng.IntN(9))
		direction := neighborDirections[rng.IntN(4)]
		plain, plainFound := NearestWindowInDirection(origin, candidates, direction, NeighborOptions{})
		wrapped, wrappedFound := NearestWindowInDirection(origin, candidates, direction, NeighborOptions{Wrap: true})
		if plainFound && (!wrappedFound || wrapped != plain) {
			t.Fatalf("iteration %d: wrap changed an answer that existed without it", iteration)
		}
	}
}

func TestPropertyVerifyClaimsSuccessOnlyForAReachedFrame(t *testing.T) {
	rng := newPropertyRand(11)
	for iteration := 0; iteration < propertyIterations; iteration++ {
		requested := randomFiniteRect(rng)
		original := randomFiniteRect(rng)
		observed := requested
		switch rng.IntN(4) {
		case 0:
			observed = original
		case 1:
			observed.Width += float64(rng.IntN(40))
		case 2:
			observed.X += float64(rng.IntN(10) - 5)
		}
		minSize := Size{Width: float64(rng.IntN(300)), Height: float64(rng.IntN(300))}
		verdict := Verify(requested, observed, original, minSize)
		reached := FramesMatch(observed, requested, ExactTolerancePoints)
		if (verdict.Outcome == Applied) != reached {
			t.Fatalf("iteration %d: outcome %v with reached=%v (requested %+v observed %+v)", iteration, verdict.Outcome, reached, requested, observed)
		}
		if verdict.Outcome == Failed && (reached || !FramesMatch(observed, original, ExactTolerancePoints)) {
			t.Fatalf("iteration %d: failed without the original frame", iteration)
		}
		if verdict.Outcome != Applied && verdict.Message == "" && verdict.Outcome == Failed {
			t.Fatalf("iteration %d: failed without a message", iteration)
		}
	}
}
