package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParsePortCommand(t *testing.T) {
	exp := []int{11940, 11941}
	got, _ := parsePortCommand("SET_PORTS 11940 11941\n")
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
