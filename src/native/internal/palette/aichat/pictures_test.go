package aichat

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/design/golden"
	"github.com/pablopunk/nvm/native/internal/fixtures"
)

// testPicture is a w by h PNG of one color, read as a chat image named name.
func testPicture(t *testing.T, name string, w, h int, shade uint8) chat.Image {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: uint8(x * 255 / w), B: uint8(y * 255 / h), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	image, err := chat.NewImage(name, buf.Bytes())
	if err != nil {
		t.Fatalf("NewImage: %v", err)
	}
	return image
}

// pictureDeps is the page dependencies with a chooser that hands over images and a clipboard that holds none.
func pictureDeps(r *rig, images ...chat.Image) Deps {
	deps := r.deps()
	deps.PickImages = func(done func([]chat.Image)) { done(images) }
	deps.PasteImage = func() (chat.Image, bool, error) { return chat.Image{}, false, nil }
	return deps
}

// TestAttachIsOfferedOnSmartOnly checks the attach control shows on the Smart tier, as the desktop app's does.
func TestAttachIsOfferedOnSmartOnly(t *testing.T) {
	r := newRig(t, nil)
	fast := r.chatWith("Fast", chat.TierFast)
	r.open(ChatView(pictureDeps(r), fast.ID))
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	if _, ok := tt.Find("Attach images"); ok {
		t.Fatal("the attach control is offered on the Fast tier")
	}

	smart := r.chatWith("Smart", chat.TierSmart)
	r.open(ChatView(pictureDeps(r), smart.ID))
	tt.Frame()
	if _, ok := tt.Find("Attach images"); !ok {
		t.Fatal("the attach control is not offered on the Smart tier")
	}
}

// TestPickedPicturesGoWithTheMessage checks pictures picked for a Smart chat show as thumbnails, can be removed, and
// are sent with the next message, which clears them.
func TestPickedPicturesGoWithTheMessage(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Pictures", chat.TierSmart)
	shot := testPicture(t, "shot.png", 64, 48, 200)
	deps := pictureDeps(r, shot)
	p := newPage(deps, record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Attach images"); err != nil {
		t.Fatalf("click Attach images: %v", err)
	}
	tt.Frame()
	if _, ok := tt.Find("Remove shot.png"); !ok || len(p.attachments) != 1 {
		t.Fatalf("the picked picture is not shown as an attachment: %d attached", len(p.attachments))
	}

	tt.Type("what is this")
	tt.Key(0, ui.KeyEnter)
	messages := mustChat(t, r, record.ID).Messages
	if len(messages) == 0 || messages[0].Text != "what is this" || len(messages[0].Images) != 1 {
		t.Fatalf("messages = %+v, want the text sent with one picture", messages)
	}
	if len(p.attachments) != 0 {
		t.Fatal("the pictures stayed attached after they were sent")
	}
}

// TestRemovingAPictureDropsIt checks the control on a thumbnail takes that picture off the next message.
func TestRemovingAPictureDropsIt(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Remove", chat.TierSmart)
	p := newPage(pictureDeps(r), record.ID)
	p.attachments = []chat.Image{testPicture(t, "one.png", 8, 8, 10), testPicture(t, "two.png", 8, 8, 90)}
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Remove one.png"); err != nil {
		t.Fatalf("click Remove one.png: %v", err)
	}
	tt.Frame()
	if len(p.attachments) != 1 || p.attachments[0].Name != "two.png" {
		t.Fatalf("attachments = %+v after removing one.png, want two.png only", p.attachments)
	}
}

// TestNinthPictureIsRefusedWithTheLimit checks the chooser's ninth picture is refused with the limit's copy, and the
// eight kept.
func TestNinthPictureIsRefusedWithTheLimit(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Limit", chat.TierSmart)
	nine := make([]chat.Image, chat.ImageLimit+1)
	for i := range nine {
		nine[i] = testPicture(t, "p.png", 4, 4, uint8(i))
	}
	p := newPage(pictureDeps(r, nine...), record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	if err := tt.Click("Attach images"); err != nil {
		t.Fatalf("click Attach images: %v", err)
	}
	tt.Frame()
	if len(p.attachments) != chat.ImageLimit || !r.said(chat.ImageLimitCopy) {
		t.Fatalf("attached %d, said %v; want %d and the limit copy", len(p.attachments), r.said(chat.ImageLimitCopy), chat.ImageLimit)
	}
}

// TestPastingAnImageAttachesIt checks a paste with no text takes the clipboard's picture, and a paste of text goes to
// the composer.
func TestPastingAnImageAttachesIt(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Paste", chat.TierFast)
	shot := testPicture(t, "clip.png", 16, 16, 40)
	deps := r.deps()
	deps.PasteImage = func() (chat.Image, bool, error) { return shot, true, nil }
	p := newPage(deps, record.ID)
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()

	tt.SetClipboard("")
	tt.Command("paste")
	if len(p.attachments) != 1 {
		t.Fatalf("attachments = %d after pasting a picture, want 1", len(p.attachments))
	}

	tt.SetClipboard("plain words")
	tt.Command("paste")
	tt.Frame()
	if p.draft != "plain words" {
		t.Fatalf("composer = %q after pasting text, want the text", p.draft)
	}
}

// TestRefusedSendKeepsThePictures checks a send refused while a reply streams keeps the pictures for the next try.
func TestRefusedSendKeepsThePictures(t *testing.T) {
	transport := fixtures.NewStreamingTransport()
	r := newRig(t, streamingClient(t, transport))
	record := r.chatWith("Busy pictures", chat.TierSmart)
	p := newPage(pictureDeps(r), record.ID)
	p.attachments = []chat.Image{testPicture(t, "wait.png", 8, 8, 70)}
	r.open(p.view())
	tt := ui.NewTester(r.palette.View, goldenWidth, goldenHeight)
	tt.Frame()
	tt.Type("one")
	tt.Key(0, ui.KeyEnter)
	reply := awaitReply(t, transport)
	writeFrame(t, reply, fixtures.StreamFrames("Looking", 24)[0])
	r.awaitReplyText(record.ID, "Looking")
	tt.Frame()

	p.attachments = []chat.Image{testPicture(t, "wait.png", 8, 8, 70)}
	tt.Type("two")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(p.attachments) != 1 || p.draft != "two" {
		t.Fatalf("attachments = %d and composer = %q after a refused send, want both kept", len(p.attachments), p.draft)
	}
}

// TestPicturesGoldens renders a Smart chat whose message carries a picture, and its composer holding two more.
func TestPicturesGoldens(t *testing.T) {
	r := newRig(t, nil)
	record := r.chatWith("Pictures", chat.TierSmart,
		chat.Message{Author: chat.AuthorUser, Text: "What is in these?", Status: chat.StatusComplete, At: goldenNow,
			Images: []chat.Image{testPicture(t, "a.png", 120, 80, 180)}},
		assistant("A landscape and a chart."))
	p := newPage(pictureDeps(r), record.ID)
	p.attachments = []chat.Image{testPicture(t, "lake.png", 96, 64, 60), testPicture(t, "chart.png", 64, 96, 230)}
	r.open(p.view())
	golden.AssertMatrix(t, "pictures", r.palette.View, pageGoldens)
}
