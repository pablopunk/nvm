package extai_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/ai/extai"
	"github.com/pablopunk/nvm/native/internal/ai/extai/extaitest"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/fixtures"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func chatBodies(rig *extaitest.Rig) []string {
	var bodies []string
	for _, request := range rig.Backend.Requests() {
		if request.Path == fixtures.ChatCompletionsPath {
			bodies = append(bodies, request.Body)
		}
	}
	return bodies
}

func TestCompleteReturnsTheReplyOnTheTierRoute(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetReply("hello there")
	completed, err := rig.Service.Complete(context.Background(), extai.Call{
		Options: api.AIOptions{Model: api.ModelFast, System: "Be brief."},
		Prompt:  "say hi",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Text != "hello there" {
		t.Fatalf("text %q; want the fixture reply", completed.Text)
	}
	bodies := chatBodies(rig)
	if len(bodies) != 1 || !strings.Contains(bodies[0], "Be brief.") || !strings.Contains(bodies[0], "say hi") {
		t.Fatalf("chat bodies %v; want the system text and the prompt", bodies)
	}
	if strings.Contains(bodies[0], `"model":"fast"`) {
		t.Fatalf("the tier leaked into the body as a model name: %s", bodies[0])
	}
	if !hasTierHeader(rig.Set.HTTP.Requests(), "fast") {
		t.Fatal("no chat request carried the fast tier header")
	}
}

func hasTierHeader(requests []fake.RecordedRequest, tier string) bool {
	for _, request := range requests {
		if strings.HasSuffix(request.URL, fixtures.ChatCompletionsPath) && request.Header.Get(backend.RoleHeader) == tier {
			return true
		}
	}
	return false
}

func TestOpenStreamsTheReplyPieceByPiece(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetReply("streamed reply")
	reply, err := rig.Service.Open(context.Background(), extai.Call{Prompt: "stream"}, extai.StreamTimeout)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reply.Close()
	var text strings.Builder
	for {
		piece, err := reply.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		text.WriteString(piece)
	}
	if text.String() != "streamed reply" {
		t.Fatalf("streamed %q; want the whole reply", text.String())
	}
	if prompt := reply.Prompt(); prompt.Role != backend.RoleUser {
		t.Fatalf("prompt role %q; want user", prompt.Role)
	}
}

func TestNotSignedInRefusesEveryCall(t *testing.T) {
	rig := extaitest.New(t)
	rig.SignOut()
	if rig.Service.Available() {
		t.Fatal("available while signed out")
	}
	if _, err := rig.Service.Complete(context.Background(), extai.Call{Prompt: "hi"}); !errors.Is(err, chat.ErrNotSignedIn) {
		t.Fatalf("complete error %v; want chat.ErrNotSignedIn", err)
	}
	if _, err := rig.Service.Decide(context.Background(), api.DecisionRequest{}); !errors.Is(err, chat.ErrNotSignedIn) {
		t.Fatalf("decide error %v; want chat.ErrNotSignedIn", err)
	}
	if len(rig.Backend.Requests()) != 0 {
		t.Fatalf("signed-out calls reached the backend: %v", rig.Backend.Requests())
	}
}

func TestUnknownTierIsRefusedWithTheOneXMessage(t *testing.T) {
	rig := extaitest.New(t)
	_, err := rig.Service.Complete(context.Background(), extai.Call{Options: api.AIOptions{Model: "huge"}, Prompt: "hi"})
	var input extai.InputError
	if !errors.As(err, &input) || string(input) != "Unsupported AI model role: huge. Use 'smart' or 'fast'." {
		t.Fatalf("error %v; want the 1.x tier message", err)
	}
}

func TestTimeoutEndsTheCallAsTimedOut(t *testing.T) {
	rig := extaitest.New(t)
	rig.Backend.SetMode(fixtures.ModeTimeout)
	result := make(chan error, 1)
	go func() {
		_, err := rig.Service.Complete(context.Background(), extai.Call{Prompt: "hang"})
		result <- err
	}()
	<-rig.Backend.Hanging()
	rig.Set.Clock.AwaitTimers(1)
	rig.Set.Clock.Advance(extai.CallTimeout)
	if err := <-result; !errors.Is(err, extai.ErrTimedOut) && !backend.IsKind(err, backend.KindTimeout) {
		t.Fatalf("error %v; want a timeout", err)
	}
}

func TestAttachmentsCarryTextAndImagesIntoThePrompt(t *testing.T) {
	rig := extaitest.New(t)
	if err := rig.Set.FS.MkdirAll("/home/ada", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rig.Set.FS.WriteFile("/home/ada/notes.md", []byte("# Notes\nHello from notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rig.Set.FS.WriteFile("/home/ada/shot.png", []byte("\x89PNG-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := rig.Service.Complete(context.Background(), extai.Call{Prompt: "read these", Options: api.AIOptions{Attachments: []api.Attachment{
		{Kind: "file", Path: "~/notes.md"},
		{Kind: "image", Path: "/home/ada/shot.png"},
	}}})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	bodies := chatBodies(rig)
	if len(bodies) != 1 || !strings.Contains(bodies[0], "Hello from notes") || !strings.Contains(bodies[0], "data:image/png;base64,") {
		t.Fatalf("chat body %v; want the note text and the PNG as a data URL", bodies)
	}
}

func TestRelativeAttachmentPathIsRefused(t *testing.T) {
	rig := extaitest.New(t)
	_, err := rig.Service.Complete(context.Background(), extai.Call{Prompt: "x", Options: api.AIOptions{Attachments: []api.Attachment{{Kind: "file", Path: "notes.md"}}}})
	var input extai.InputError
	if !errors.As(err, &input) || !strings.Contains(string(input), "must be absolute") {
		t.Fatalf("error %v; want an absolute-path refusal", err)
	}
}

func intPtr(value int) *int { return &value }

func TestDecideReturnsTypedAnswersAndKeepsZeroMinimum(t *testing.T) {
	rig := extaitest.New(t)
	result, err := rig.Service.Decide(context.Background(), api.DecisionRequest{
		Input: json.RawMessage(`"the input"`),
		Questions: []api.DecisionQuestion{
			{ID: "spam", Type: api.DecisionPredicate, Question: "Is it spam?"},
			{ID: "level", Type: api.DecisionScore, Question: "How urgent?", Min: intPtr(0), Max: intPtr(5)},
		},
	})
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if result.Answers["spam"].Type != api.DecisionPredicate || result.Answers["level"].Type != api.DecisionScore {
		t.Fatalf("answers %+v; want a predicate and a score", result.Answers)
	}
	var sent []map[string]any
	for _, request := range rig.Backend.Requests() {
		if request.Path == fixtures.DecisionsPath {
			var body struct {
				Input     string           `json:"input"`
				Questions []map[string]any `json:"questions"`
			}
			if err := json.Unmarshal([]byte(request.Body), &body); err != nil {
				t.Fatalf("decision body: %v", err)
			}
			sent = body.Questions
			if body.Input != "the input" {
				t.Fatalf("input %q; want the shared input", body.Input)
			}
		}
	}
	if len(sent) != 2 || sent[1]["min"] != float64(0) {
		t.Fatalf("questions sent %v; want the score minimum 0 kept", sent)
	}
}

func TestDecideRefusesTooManyAndUnsupportedImages(t *testing.T) {
	rig := extaitest.New(t)
	var five []api.Attachment
	for i := 0; i < 5; i++ {
		five = append(five, api.Attachment{Kind: "image", DataURL: "data:image/png;base64,AAAA"})
	}
	_, err := rig.Service.Decide(context.Background(), api.DecisionRequest{Attachments: five, Questions: []api.DecisionQuestion{{ID: "q", Type: api.DecisionPredicate, Question: "?"}}})
	if err == nil || err.Error() != "Decisions accept at most 4 images; 5 were attached." {
		t.Fatalf("error %v; want the 1.x image count message", err)
	}
	_, err = rig.Service.Decide(context.Background(), api.DecisionRequest{
		Attachments: []api.Attachment{{Kind: "image", DataURL: "data:image/gif;base64,AAAA"}},
		Questions:   []api.DecisionQuestion{{ID: "q", Type: api.DecisionPredicate, Question: "?"}},
	})
	if err == nil || err.Error() != "Decision image 1 is image/gif; use PNG, JPEG or WebP." {
		t.Fatalf("error %v; want the 1.x image type message", err)
	}
}

func TestPrepareWarmsTheTierRoute(t *testing.T) {
	rig := extaitest.New(t)
	if err := rig.Service.Warm(context.Background(), api.AIOptions{Model: api.ModelSmart}); err != nil {
		t.Fatalf("warm: %v", err)
	}
	if len(rig.Backend.Requests()) == 0 {
		t.Fatal("warm sent no route lookup")
	}
	if len(chatBodies(rig)) != 0 {
		t.Fatal("warm sent a reply request")
	}
}
