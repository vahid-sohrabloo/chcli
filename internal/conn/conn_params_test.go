package conn

import (
	"testing"
)

func TestUintParamHelperProducesNamedParameter(t *testing.T) {
	p := UintParam("rounding", uint32(60))
	s := p()
	if s.Name != "rounding" {
		t.Errorf("param name = %q, want rounding", s.Name)
	}
}

func TestStringParamHelperProducesNamedParameter(t *testing.T) {
	p := StringParam("db", "events")
	s := p()
	if s.Name != "db" {
		t.Errorf("param name = %q, want db", s.Name)
	}
}

func TestIntParamHelperProducesNamedParameter(t *testing.T) {
	p := IntParam("n", -5)
	s := p()
	if s.Name != "n" {
		t.Errorf("param name = %q, want n", s.Name)
	}
}
