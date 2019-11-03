package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestParsePortCommand(t *testing.T) {
	exp := []int{11940, 11941, 11942}
	got, _ := parseManagementPortList([]string{"11940", "11941", "11942"})
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandZero(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parseManagementPortList([]string{"0", "11941"})
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}

func TestParsePortCommandPortRange(t *testing.T) {
	exp := errors.New("INVALID_PARAMETER")
	_, got := parseManagementPortList([]string{"11940", "11941", "65536"})
	if got.Error() != exp.Error() {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}
