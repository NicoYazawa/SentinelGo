package utils

import "testing"

func TestMenuTypeString(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{1, "M"},
		{2, "C"},
		{3, "F"},
		{0, "0"},
		{99, "99"},
	}
	for _, c := range cases {
		if got := MenuTypeString(c.in); got != c.want {
			t.Errorf("MenuTypeString(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildMenuTree(t *testing.T) {
	menus := []MenuInput{
		{ID: 1, ParentID: 0, Name: "Root", Sort: 2},
		{ID: 2, ParentID: 1, Name: "ChildB", Sort: 2, Type: "C"},
		{ID: 3, ParentID: 1, Name: "ChildA", Sort: 1, Type: "C"},
		{ID: 4, ParentID: 0, Name: "Root2", Sort: 1},
		{ID: 5, ParentID: 99, Name: "Orphan", Sort: 0}, // parent missing
	}

	tree := BuildMenuTree(menus)

	// top-level nodes sorted by Sort: Root2(1) then Root(2)
	if len(tree) != 2 {
		t.Fatalf("expected 2 top nodes, got %d", len(tree))
	}
	if tree[0].Name != "Root2" || tree[1].Name != "Root" {
		t.Errorf("top order wrong: %s, %s", tree[0].Name, tree[1].Name)
	}

	// Root's children sorted: ChildA(1) then ChildB(2)
	root := tree[1]
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(root.Children))
	}
	if root.Children[0].Name != "ChildA" || root.Children[1].Name != "ChildB" {
		t.Errorf("child order wrong: %s, %s", root.Children[0].Name, root.Children[1].Name)
	}

	// empty list
	if got := BuildMenuTree([]MenuInput{}); len(got) != 0 {
		t.Errorf("expected empty tree, got %d", len(got))
	}
}
