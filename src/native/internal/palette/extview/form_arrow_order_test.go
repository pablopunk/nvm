package extview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

func ticketFormSpec() api.FormSpec {
	return api.FormSpec{Fields: []api.Field{
		api.FieldOf("name", api.FieldText, "Name"),
		api.FieldOf("bio", api.FieldTextarea, "Bio"),
		api.FieldOf("token", api.FieldPassword, "API token"),
		api.FieldOf("email", api.FieldEmail, "Email"),
		api.FieldOf("site", api.FieldText, "Website"),
	}}
}

// Arrows that arrive right after a field's Set move the selection from the field's own row in order: three Downs from
// Name land on Email whatever frames fall between the keys.
func TestArrowsAfterSettingAFieldKeepTheirOrderWhateverFramesFallBetween(t *testing.T) {
	for mask := range 1 << 4 {
		f := openFormInPalette(t, ticketFormSpec())
		f.key(0, ui.KeyEnter)
		f.typeText("Ada")
		f.key(0, ui.KeyEnter)
		f.key(0, ui.KeyEnter)
		f.tt.Type(" Lovelace")
		step := 0
		frameIfAsked := func() {
			if mask&(1<<step) != 0 {
				f.tt.Frame()
			}
			step++
		}
		f.tt.Key(0, ui.KeyEnter) // set Name
		frameIfAsked()
		for range 3 {
			f.tt.Key(0, ui.KeyDown)
			frameIfAsked()
		}
		f.tt.Key(0, ui.KeyEnter)
		f.tt.Frame()
		if got := f.r.session.form.editing; got != "email" {
			t.Fatalf("frames %04b: the field edited is %q, want email", mask, got)
		}
		f.expectValue("name", "Ada Lovelace")
	}
}
