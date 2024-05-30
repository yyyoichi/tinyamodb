package btree_test

import (
	"testing"

	"github.com/yyyoichi/tinyamodb/btree"
)

// テスト用のOrdered型
type Int int

func lessInt(a, b Int) bool {
	return a < b
}

func TestBTreeG_InsertAndGet(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	for _, item := range items {
		if got, found := tree.Get(item); !found || got != item {
			t.Fatalf("expected to find %v, but got %v", item, got)
		}
	}

	if got := tree.Len(); got != len(items) {
		t.Fatalf("expected length %v, but got %v", len(items), got)
	}
}

func TestBTreeG_Delete(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	for _, item := range items {
		if _, found := tree.Delete(item); !found {
			t.Fatalf("expected to delete %v", item)
		}
		if _, found := tree.Get(item); found {
			t.Fatalf("expected not to find %v after deletion", item)
		}
	}

	if got := tree.Len(); got != 0 {
		t.Fatalf("expected length 0, but got %v", got)
	}
}

func TestBTreeG_MinMax(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	if min, found := tree.Min(); !found || min != 5 {
		t.Fatalf("expected min 5, but got %v", min)
	}

	if max, found := tree.Max(); !found || max != 30 {
		t.Fatalf("expected max 30, but got %v", max)
	}
}

func TestBTreeG_Ascend(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	var result []Int
	tree.Ascend(func(item Int) bool {
		result = append(result, item)
		return true
	})

	expected := []Int{5, 6, 10, 15, 20, 25, 30}
	for i, item := range expected {
		if result[i] != item {
			t.Fatalf("expected %v at index %v, but got %v", item, i, result[i])
		}
	}
}

func TestBTreeG_Descend(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	var result []Int
	tree.Descend(func(item Int) bool {
		result = append(result, item)
		return true
	})

	expected := []Int{30, 25, 20, 15, 10, 6, 5}
	for i, item := range expected {
		if result[i] != item {
			t.Fatalf("expected %v at index %v, but got %v", item, i, result[i])
		}
	}
}

func TestBTreeG_Clone(t *testing.T) {
	tree := btree.NewG(3, lessInt)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	clone := tree.Clone()

	for _, item := range items {
		if got, found := clone.Get(item); !found || got != item {
			t.Fatalf("expected to find %v in clone, but got %v", item, got)
		}
	}

	if got := clone.Len(); got != len(items) {
		t.Fatalf("expected clone length %v, but got %v", len(items), got)
	}
}

func TestBTreeG_WithFreeList(t *testing.T) {
	freelist := &btree.FreeListG[Int]{}
	tree := btree.NewWithFreeListG(3, lessInt, freelist)

	items := []Int{10, 20, 5, 6, 15, 30, 25}
	for _, item := range items {
		tree.ReplaceOrInsert(item)
	}

	tree.Clear(true)

	if got := tree.Len(); got != 0 {
		t.Fatalf("expected length 0 after clear, but got %v", got)
	}

	// if len(freelist) == 0 {
	// 	t.Fatalf("expected freelist to have nodes after clear")
	// }
}
