package model

import "testing"

func TestMenuMethods(t *testing.T) {
	m := Menu{BaseModel: BaseModel{ID: 5}, ParentID: 2, Name: "n", Path: "/p", Type: 1, Sort: 3, Status: 1}
	if m.GetID() != uint64(5) {
		t.Errorf("GetID: %d", m.GetID())
	}
	if m.GetParentID() != uint64(2) {
		t.Errorf("GetParentID: %d", m.GetParentID())
	}
	if m.GetName() != "n" {
		t.Errorf("GetName: %s", m.GetName())
	}
	if m.GetPath() != "/p" {
		t.Errorf("GetPath: %s", m.GetPath())
	}
	if m.GetType() != "M" {
		t.Errorf("GetType: %s", m.GetType())
	}
	if m.GetSort() != 3 {
		t.Errorf("GetSort: %d", m.GetSort())
	}

	// unknown type maps to its numeric string; empty path handled
	m2 := Menu{Type: 0}
	if m2.GetType() != "0" {
		t.Errorf("GetType(0): %s", m2.GetType())
	}
	if m2.GetPath() != "" {
		t.Errorf("GetPath empty: %s", m2.GetPath())
	}
}
