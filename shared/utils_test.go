package shared

import (
	"testing"
)

func TestSnakeCase(t *testing.T) {
	if SnakeCase("testing-string") != "testing_string" {
		t.Fail()
	}

	if SnakeCase("TestingString") != "testing_string" {
		t.Fail()
	}

	if SnakeCase("Testing_String") != "testing__string" {
		t.Fail()
	}
}

func TestParameterizeString(t *testing.T) {
	if ParameterizeString("testing-string") != "testing_string" {
		t.Fail()
	}

	if ParameterizeString("TestingString") != "testingstring" {
		t.Fail()
	}

	if ParameterizeString("Testing-String") != "testing_string" {
		t.Fail()
	}
}
