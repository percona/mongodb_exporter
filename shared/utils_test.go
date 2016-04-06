package shared

import (
	"testing"
)

func Test_SnakeCase(t *testing.T) {
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

func Test_ParameterizeString(t *testing.T) {
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
