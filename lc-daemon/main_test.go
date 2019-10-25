package main

import (
	"errors"
	"reflect"
	"testing"
)

//Test for the function ParsePortCommand till line 72
func TestParsePortCommand(t *testing.T) {
	exp := []int{11940, 11941}
	got, _ := parsePortCommand("SET_PORTS 11940 11941\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandDuplicate(t *testing.T) {
	exp := []int{11940, 11941, 11942}
	got, _ := parsePortCommand("SET_PORTS 11940 11941 11941 11942\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandWhitespace(t *testing.T) {
	exp := []int{11940, 11941}
	got, _ := parsePortCommand("SET_PORTS\t\t     11940 \t\t11941\t\r\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandString(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parsePortCommand("SET_PORTS a b 11941\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandZero(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parsePortCommand("SET_PORTS 0 11941\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandNoParameter(t *testing.T) {
	exp := errors.New("MISSING_PARAMETER")
	_, got := parsePortCommand("SET_PORTS \n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandMalformedCommand(t *testing.T) {
	exp := errors.New("NOT_SUPPORTED")
	_, got := parsePortCommand("SET_PORTSS 11940 11941\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandPortRange(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parsePortCommand("SET_PORTS 11940 11941 65536\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

//Test for the function ParseDisconnectCommand till 121
func TestParseDisconnectCommand(t *testing.T) {
	exp := "foo"
	got, _ := parseDisconnectCommand("DISCONNECT foo\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParseDisconnectCommandMultiple(t *testing.T) {
	exp := "foo"
	got, _ := parseDisconnectCommand("DISCONNECT foo root\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParseDisconnectCommandWhitespace(t *testing.T) {
	exp := "foo"
	got, _ := parseDisconnectCommand("DISCONNECT\t\t    foo\t\r\n")
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParseDisconnectCommandMalformed(t *testing.T) {
	exp := errors.New("NOT_SUPPORTED")
	_, got := parseDisconnectCommand("DISCONNECTT foo\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParseDisconnectCommandNoParameter(t *testing.T) {
	exp := errors.New("MISSING_PARAMETER")
	_, got := parseDisconnectCommand("DISCONNECT \n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParseDisconnectCommandParameterBad(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parseDisconnectCommand("DISCONNECT foo@daemon\n")
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}
