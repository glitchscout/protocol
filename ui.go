package protocol

import "slices"

// The standard UI element types (observation ui_element.type). Every SDK reports one of these; see the README's
// "UI elements" section for what each means.
const (
	UIButton    = "button"     // pressed to do something: a button, a menu entry, a link
	UICheckbox  = "checkbox"   // toggled on and off; its value is a boolean
	UIOption    = "option"     // a dropdown; its text and value are the selected option
	UISlider    = "slider"     // a number in a range, a spin box included; its value is the number
	UITextInput = "text_input" // a single- or multi-line text field; its value is the text
	UILabel     = "label"      // text a player reads and can't operate
	UIControl   = "control"    // any other control the game marks (a tab, a panel)
)

// UITypes is every standard UI element type, in the order observation.schema.json lists them.
var UITypes = []string{UIButton, UICheckbox, UIOption, UISlider, UITextInput, UILabel, UIControl}

// UIInteractiveTypes are the standard types a player operates: every type but label and control.
var UIInteractiveTypes = []string{UIButton, UICheckbox, UIOption, UISlider, UITextInput}

// IsUIType reports whether typ is one of UITypes.
func IsUIType(typ string) bool { return slices.Contains(UITypes, typ) }
