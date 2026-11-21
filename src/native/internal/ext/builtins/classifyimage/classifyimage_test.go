package classifyimage

import (
	"errors"
	"strings"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// fakeFiles answers Metadata from sizes; a path missing from sizes cannot be read.
type fakeFiles struct {
	api.Files
	sizes map[string]int64
}

func (f fakeFiles) Metadata(path string) (api.FileRef, error) {
	size, ok := f.sizes[path]
	if !ok {
		return api.FileRef{}, errors.New("no such file")
	}
	return api.FileRef{Path: path, Size: size}, nil
}

// fakeAI records the decision requests it receives and answers with one result or one error.
type fakeAI struct {
	api.AI
	available bool
	err       error
	answers   map[string]api.DecisionAnswer
	requests  []api.DecisionRequest
}

func (a *fakeAI) IsAvailable() (bool, error) { return a.available, nil }

func (a *fakeAI) Chats() api.AIChats { return nil }

func (a *fakeAI) Decide(request api.DecisionRequest) (api.DecisionResult, error) {
	a.requests = append(a.requests, request)
	if a.err != nil {
		return api.DecisionResult{}, a.err
	}
	return api.DecisionResult{Answers: a.answers}, nil
}

func (a *fakeAI) Attachments() api.AttachmentBuilder { return fakeAttachments{} }

// fakeAttachments builds image attachments by path, as the host does.
type fakeAttachments struct{ api.AttachmentBuilder }

func (fakeAttachments) Image(path, title string, _ bool) api.Attachment {
	return api.Attachment{Kind: "image", Path: path, Title: title}
}

// fakeContext serves the AI and the files; every other service is absent.
type fakeContext struct {
	api.Context
	ai    *fakeAI
	files fakeFiles
}

func (c fakeContext) AI() api.AI           { return c.ai }
func (c fakeContext) Desktop() api.Desktop { return fakeDesktop{files: c.files} }

// fakeDesktop serves the files only.
type fakeDesktop struct {
	api.Desktop
	files fakeFiles
}

func (d fakeDesktop) Files() api.Files { return d.files }

const picture = "/Users/me/Pictures/cat.jpg"

// signedInContext is a context whose account is signed in, with picture of size bytes on disk.
func signedInContext(size int64) (fakeContext, *fakeAI) {
	ai := &fakeAI{available: true, answers: map[string]api.DecisionAnswer{
		"category": {Type: api.DecisionChoice, Choice: "photograph"},
	}}
	return fakeContext{ai: ai, files: fakeFiles{sizes: map[string]int64{picture: size}}}, ai
}

// submit runs the form's submit with the picture at path.
func submit(t *testing.T, ctx fakeContext, path string) (api.Result, error) {
	t.Helper()
	return Extension{}.RunAction(ctx, api.Invocation{
		Ref:  api.ActionRef{ID: classifyAction},
		Form: api.FormValues{"image": api.TextValue(path)},
	})
}

// TestRowOpensThePictureForm checks that the row's primary action opens a form with one picture field, typed and
// filtered as the native picker was, whose submit classifies it.
func TestRowOpensThePictureForm(t *testing.T) {
	result, err := Extension{}.RunAction(fakeContext{}, api.Invocation{Ref: api.ActionRef{ID: chooseAction}})
	if err != nil || result.View == nil || result.View.Form == nil || result.View.Title != Title {
		t.Fatalf("result = %+v, %v; want the form titled %q", result, err, Title)
	}
	field := result.View.Form.Fields[0]
	if field.ID != "image" || field.Type != api.FieldFile || field.FilterName != "Images" || field.ButtonLabel != "Classify Image" {
		t.Fatalf("field = %+v", field)
	}
	if strings.Join(field.Extensions, ",") != "png,jpg,jpeg" {
		t.Fatalf("extensions = %v, want the native picker's types", field.Extensions)
	}
}

// TestRowMatchesItsTitleAndKeywords checks that the row answers a query for its title or keywords and nothing else.
func TestRowMatchesItsTitleAndKeywords(t *testing.T) {
	for _, query := range []string{"classify", "screenshot", "image"} {
		if rows, _ := (Extension{}).SearchItems(fakeContext{}, query); len(rows) != 1 {
			t.Fatalf("query %q: rows = %v, want the classify row", query, rows)
		}
	}
	if rows, _ := (Extension{}).SearchItems(fakeContext{}, "calendar"); len(rows) != 0 {
		t.Fatalf("calendar matched %v", rows)
	}
	rows, _ := (Extension{}).SearchItems(fakeContext{}, "image")
	if rows[0].Section != search.KindCommands || rows[0].LearnKey != "cmd:"+Title {
		t.Fatalf("row = %+v, want a command row that learns by its title", rows[0])
	}
}

// TestClassifyListsTheCategoryAndCopiesIt sends the picture to the decision route and lists the category it answers,
// whose primary action copies it.
func TestClassifyListsTheCategoryAndCopiesIt(t *testing.T) {
	ctx, ai := signedInContext(1024)
	result, err := submit(t, ctx, picture)
	if err != nil || result.View == nil || result.View.Title != ResultTitle {
		t.Fatalf("result = %+v, %v; want the list %q", result, err, ResultTitle)
	}
	row := result.View.Items[0]
	if row.Title != "photograph" || row.Primary == nil || row.Primary.Kind != api.ActionCopyText || row.Primary.Text != "photograph" {
		t.Fatalf("row = %+v, want the category with a copy action", row)
	}
	if len(ai.requests) != 1 {
		t.Fatalf("decision calls = %d, want one", len(ai.requests))
	}
	request := ai.requests[0]
	if len(request.Attachments) != 1 || request.Attachments[0].Path != picture || request.Attachments[0].Kind != "image" {
		t.Fatalf("attachments = %+v, want the picture", request.Attachments)
	}
	question := request.Questions[0]
	if question.ID != "category" || question.Type != api.DecisionChoice || len(question.Options) != 5 {
		t.Fatalf("question = %+v", question)
	}
}

// TestClassifyRefusesWhatTheRouteCannotTake checks the refusals, each before any model call.
func TestClassifyRefusesWhatTheRouteCannotTake(t *testing.T) {
	cases := []struct {
		name string
		path string
		size int64
		want error
	}{
		{"another format", "/Users/me/Pictures/anim.gif", 1024, errNoImage},
		{"unreadable", "/Users/me/Pictures/missing.png", 0, errNoImage},
		{"too large", picture, MaximumImageBytes + 1, errImageTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, ai := signedInContext(tc.size)
			if _, err := submit(t, ctx, tc.path); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(ai.requests) != 0 {
				t.Fatalf("a refused picture reached the model: %d calls", len(ai.requests))
			}
		})
	}
}

// TestClassifyAcceptsTheLargestPictureTheRouteTakes checks the limit is inclusive: 2 MiB decoded is allowed.
func TestClassifyAcceptsTheLargestPictureTheRouteTakes(t *testing.T) {
	ctx, _ := signedInContext(MaximumImageBytes)
	if _, err := submit(t, ctx, picture); err != nil {
		t.Fatalf("err = %v, want the picture accepted", err)
	}
}

// TestClassifyNeedsASignedInAccount checks the sign-in copy, before the picture is read.
func TestClassifyNeedsASignedInAccount(t *testing.T) {
	ctx, ai := signedInContext(1024)
	ai.available = false
	if _, err := submit(t, ctx, picture); !errors.Is(err, errNotSignedIn) {
		t.Fatalf("err = %v, want the sign-in copy", err)
	}
	if len(ai.requests) != 0 {
		t.Fatal("a signed-out account reached the model")
	}
}

// TestClassifyFailsWithoutACategory checks that an answer with no category is the failure copy.
func TestClassifyFailsWithoutACategory(t *testing.T) {
	ctx, ai := signedInContext(1024)
	ai.answers = map[string]api.DecisionAnswer{"category": {Type: api.DecisionPredicate, Flag: true}}
	if _, err := submit(t, ctx, picture); !errors.Is(err, errNoCategory) {
		t.Fatalf("err = %v, want the classification failure", err)
	}
}

// TestClassifyPassesTheModelsFailureThrough checks that a failed decision reports its own error.
func TestClassifyPassesTheModelsFailureThrough(t *testing.T) {
	ctx, ai := signedInContext(1024)
	ai.err = errors.New("the model is busy")
	if _, err := submit(t, ctx, picture); err == nil || err.Error() != "the model is busy" {
		t.Fatalf("err = %v, want the model's failure", err)
	}
}
