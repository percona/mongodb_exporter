package shared

import (
	"testing"
)

func Test_ParseEnabledGroups(t *testing.T) {
	ParseEnabledGroups("a, b,  c")
	if !EnabledGroups["a"] {
		t.Error("a was not loaded.")
	}
	if !EnabledGroups["b"] {
		t.Error("b was not loaded.")
	}
	if !EnabledGroups["c"] {
		t.Error("c was not loaded.")
	}
}
