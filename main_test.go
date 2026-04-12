package main

import (
	"testing"
)

func TestFetchInfo(t *testing.T) {
	pkgs, err := fetchinfo([]string{"yay"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected results, got none")
	}

	if pkgs[0].Name != "yay" {
		t.Errorf("expected yay, go %s", pkgs[0].Name)
	}
}
