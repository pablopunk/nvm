package api

import "testing"

func TestEverySixteenFieldTypeAcceptsAnEmptyOptionalValue(t *testing.T) {
	if got := len(AllFieldTypes()); got != 16 {
		t.Fatalf("AllFieldTypes has %d entries, want 16", got)
	}
	for _, fieldType := range AllFieldTypes() {
		field := Field{ID: "f", Type: fieldType}
		if err := ValidateFieldValue(field, FormValue{}); err != nil {
			t.Errorf("type %s rejects an empty optional value: %v", fieldType, err)
		}
	}
}

func TestValidateFieldValueAcrossTypes(t *testing.T) {
	zero := 0.0
	ten := 10.0
	tests := []struct {
		name    string
		field   Field
		value   FormValue
		wantErr bool
	}{
		{name: "text ok", field: Field{ID: "t", Type: FieldText}, value: TextValue("hello")},
		{name: "text required blank", field: Field{ID: "t", Type: FieldText, Required: true}, value: TextValue("  "), wantErr: true},
		{name: "text too long in runes", field: Field{ID: "t", Type: FieldText, Rules: &Rules{MaxLength: 3}}, value: TextValue("héllo"), wantErr: true},
		{name: "text pattern full match", field: Field{ID: "t", Type: FieldText, Rules: &Rules{Pattern: "[a-z]+"}}, value: TextValue("abc1"), wantErr: true},
		{name: "textarea ok", field: Field{ID: "t", Type: FieldTextarea}, value: TextValue("line one\nline two")},
		{name: "password required", field: Field{ID: "p", Type: FieldPassword, Required: true}, value: TextValue(""), wantErr: true},
		{name: "email ok", field: Field{ID: "e", Type: FieldEmail}, value: TextValue("ada@example.com")},
		{name: "email display form rejected", field: Field{ID: "e", Type: FieldEmail}, value: TextValue("Ada <ada@example.com>"), wantErr: true},
		{name: "url ok", field: Field{ID: "u", Type: FieldURL}, value: TextValue("https://example.com/x")},
		{name: "url file scheme rejected", field: Field{ID: "u", Type: FieldURL}, value: TextValue("file:///etc/passwd"), wantErr: true},
		{name: "number ok", field: Field{ID: "n", Type: FieldNumber, Rules: &Rules{Min: &zero, Max: &ten}}, value: TextValue("7.5")},
		{name: "number above max", field: Field{ID: "n", Type: FieldNumber, Rules: &Rules{Max: &ten}}, value: TextValue("11"), wantErr: true},
		{name: "number not numeric", field: Field{ID: "n", Type: FieldNumber}, value: TextValue("seven"), wantErr: true},
		{name: "date ok", field: Field{ID: "d", Type: FieldDate}, value: TextValue("2026-11-03")},
		{name: "date wrong format", field: Field{ID: "d", Type: FieldDate}, value: TextValue("03/11/2026"), wantErr: true},
		{name: "checkbox ok", field: Field{ID: "c", Type: FieldCheckbox}, value: FlagValue(true)},
		{name: "checkbox required unchecked", field: Field{ID: "c", Type: FieldCheckbox, Required: true}, value: FlagValue(false), wantErr: true},
		{name: "dropdown option ok", field: Field{ID: "s", Type: FieldDropdown, Options: []Option{{Title: "A", Value: "a"}}}, value: TextValue("a")},
		{name: "dropdown unknown option", field: Field{ID: "s", Type: FieldDropdown, Options: []Option{{Title: "A", Value: "a"}}}, value: TextValue("z"), wantErr: true},
		{name: "select unknown option", field: Field{ID: "s", Type: FieldSelect, Options: []Option{{Title: "A", Value: "a"}}}, value: TextValue("z"), wantErr: true},
		{name: "multiselect over max items", field: Field{ID: "m", Type: FieldMultiselect, Options: []Option{{Title: "A", Value: "a"}, {Title: "B", Value: "b"}}, Rules: &Rules{MaxItems: 1}}, value: ListValue("a", "b"), wantErr: true},
		{name: "multiselect required empty", field: Field{ID: "m", Type: FieldMultiselect, Required: true, Options: []Option{{Title: "A", Value: "a"}}}, value: ListValue(), wantErr: true},
		{name: "file extension ok", field: Field{ID: "f", Type: FieldFile, Extensions: []string{"png", ".jpg"}}, value: TextValue("/tmp/shot.PNG")},
		{name: "file extension rejected", field: Field{ID: "f", Type: FieldFile, Extensions: []string{"png"}}, value: TextValue("/tmp/notes.md"), wantErr: true},
		{name: "folder accepts any path", field: Field{ID: "d", Type: FieldFolder, Extensions: []string{"png"}}, value: TextValue("/tmp/projects")},
		{name: "files allowed list", field: Field{ID: "fs", Type: FieldFiles, Extensions: []string{"md"}}, value: ListValue("a.md", "b.txt"), wantErr: true},
		{name: "description takes no value", field: Field{ID: "x", Type: FieldDescription}, value: TextValue("oops"), wantErr: true},
		{name: "separator takes no value", field: Field{ID: "x", Type: FieldSeparator}, value: FlagValue(true), wantErr: true},
		{name: "wrong kind rejected", field: Field{ID: "t", Type: FieldText}, value: FlagValue(true), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFieldValue(tt.field, tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateFieldValue error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
