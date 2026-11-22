package extview

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

// validate checks every field the way the host does before a submit runs, marks the failures, and returns the first
// failing field's id, or "" when every value is valid.
func (f *formState) validate() string {
	f.errors = map[string]string{}
	first := ""
	for _, field := range f.spec.Fields {
		if !takesValue(field.Type) || field.Disabled {
			continue
		}
		if err := api.ValidateFieldValue(field, f.value(field)); err != nil {
			f.errors[field.ID] = fieldErrorCopy(field, err)
			if first == "" {
				first = field.ID
			}
		}
	}
	return first
}

// saveChange runs the submit action after a value changed, on a form that saves on change. The form stays open: only
// the result's toast or failure is shown, since a setting saved in place has nowhere to go. An invalid value is marked
// on its row and nothing is saved until it is fixed.
func (f *formState) saveChange(h search.Host) {
	if !f.spec.SaveOnChange || f.spec.Submit == nil || f.spec.Submit.Ref == nil {
		return
	}
	if f.validate() != "" {
		h.Refresh()
		return
	}
	s := f.sess
	if s.deps.Invoke == nil {
		h.Toast(unavailableMessage)
		return
	}
	s.deps.Invoke(s.extID, api.Invocation{Ref: *f.spec.Submit.Ref, Form: f.submittedValues()}, func(result api.Result, failure *api.Failure) {
		if result.DataChanged {
			s.revalidateMutation()
		}
		s.showToast(h, s.extID, "", result.Toast)
		if failure != nil {
			h.Toast(failure.Message)
		}
	})
}
