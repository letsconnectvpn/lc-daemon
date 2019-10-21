package main

import (
	"reflect"
	"testing"
)

func TestParsePortCommand(t *testing.T) {
	got, _ := parsePortCommand("SET_OPENVPN_MANAGEMENT_PORT_LIST 11940 11941\n")
	exp := []int{11940, 11941}
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("Got: %v, Wanted: %v", got, exp)
	}
}
