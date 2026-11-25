package protocol

import (
	"encoding/json"
	"slices"
	"testing"
)

// The Go list is the schema's: ui_element.type lists the standard types as examples, in the same order.
func TestUITypesAreTheSchemas(t *testing.T) {
	b, err := FS.ReadFile("schemas/observation.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Defs struct {
			UIElement struct {
				Properties struct {
					Type struct {
						Examples []string `json:"examples"`
					} `json:"type"`
				} `json:"properties"`
			} `json:"ui_element"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if got := s.Defs.UIElement.Properties.Type.Examples; !slices.Equal(got, UITypes) {
		t.Fatalf("schema ui_element.type examples = %v, want UITypes %v", got, UITypes)
	}
	want := []string{"button", "checkbox", "option", "slider", "text_input", "label", "control"}
	if !slices.Equal(UITypes, want) {
		t.Fatalf("UITypes = %v, want %v", UITypes, want)
	}
}

// The interactive types are the standard ones a player operates: every type but label and control.
func TestUIInteractiveTypes(t *testing.T) {
	want := []string{UIButton, UICheckbox, UIOption, UISlider, UITextInput}
	if !slices.Equal(UIInteractiveTypes, want) {
		t.Fatalf("UIInteractiveTypes = %v, want %v", UIInteractiveTypes, want)
	}
	for _, typ := range UITypes {
		if !IsUIType(typ) {
			t.Errorf("IsUIType(%q) = false", typ)
		}
	}
	for _, typ := range []string{"Button", "UButton", "menu_item", "link", ""} {
		if IsUIType(typ) {
			t.Errorf("IsUIType(%q) = true", typ)
		}
	}
}

// A valid fixture reports one element of each standard type, so SDK authors have an example of each.
func TestAValidFixtureHasEveryUIType(t *testing.T) {
	var msg struct {
		Payload struct {
			UIElements []struct {
				Type string `json:"type"`
			} `json:"ui_elements"`
		} `json:"payload"`
	}
	b, err := FS.ReadFile("fixtures/valid/OBSERVATION/ui-types.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range msg.Payload.UIElements {
		seen[e.Type] = true
	}
	for _, typ := range UITypes {
		if !seen[typ] {
			t.Errorf("ui-types.json has no %q element", typ)
		}
	}
}

// A valid fixture gives a slider, a text field and a dropdown the label a player reads beside them, apart from their
// text and value, and gives a label element none.
func TestAValidFixtureLabelsItsSliderFieldAndDropdown(t *testing.T) {
	var msg struct {
		Payload struct {
			UIElements []struct {
				Type  string  `json:"type"`
				Label *string `json:"label"`
			} `json:"ui_elements"`
		} `json:"payload"`
	}
	b, err := FS.ReadFile("fixtures/valid/OBSERVATION/ui-labels.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &msg); err != nil {
		t.Fatal(err)
	}
	labelled := map[string]bool{}
	for _, e := range msg.Payload.UIElements {
		if e.Label != nil {
			labelled[e.Type] = true
		}
	}
	for _, typ := range []string{UISlider, UITextInput, UIOption} {
		if !labelled[typ] {
			t.Errorf("ui-labels.json has no labelled %q", typ)
		}
	}
	if labelled[UILabel] {
		t.Error("ui-labels.json gives a label element a label")
	}
}
