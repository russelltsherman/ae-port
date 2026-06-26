package seeds

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"ok", "buy milk", nil},
		{"trims to ok", "  hi  ", nil},
		{"empty", "", ErrNameRequired},
		{"whitespace only", "   ", ErrNameRequired},
		{"too long", strings.Repeat("x", MaxNameLen+1), ErrNameTooLong},
		{"max length ok", strings.Repeat("x", MaxNameLen), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidateName(c.in); !errors.Is(got, c.want) {
				t.Fatalf("ValidateName(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestStoreCreate(t *testing.T) {
	s := NewStore()

	it, err := s.Create("  Write tests  ")
	if err != nil {
		t.Fatalf("Create: unexpected error %v", err)
	}
	if it.ID != 1 {
		t.Errorf("first item ID = %d, want 1", it.ID)
	}
	if it.Name != "Write tests" {
		t.Errorf("Name = %q, want trimmed %q", it.Name, "Write tests")
	}

	// IDs increment.
	it2, err := s.Create("Second")
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	if it2.ID != 2 {
		t.Errorf("second item ID = %d, want 2", it2.ID)
	}
}

func TestStoreCreateValidationAndDuplicates(t *testing.T) {
	s := NewStore()

	if _, err := s.Create(""); !errors.Is(err, ErrNameRequired) {
		t.Errorf("empty name err = %v, want ErrNameRequired", err)
	}
	if _, err := s.Create(strings.Repeat("y", MaxNameLen+1)); !errors.Is(err, ErrNameTooLong) {
		t.Errorf("long name err = %v, want ErrNameTooLong", err)
	}

	if _, err := s.Create("Unique"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Duplicate is case-insensitive and trim-insensitive.
	if _, err := s.Create("  unique "); !errors.Is(err, ErrDuplicate) {
		t.Errorf("duplicate err = %v, want ErrDuplicate", err)
	}
}

func TestStoreGet(t *testing.T) {
	s := NewStore()
	created, _ := s.Create("findme")

	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get existing: %v", err)
	}
	if got.Name != "findme" {
		t.Errorf("Get Name = %q, want %q", got.Name, "findme")
	}

	if _, err := s.Get(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing err = %v, want ErrNotFound", err)
	}
}

func TestStoreListIsSortedByID(t *testing.T) {
	s := NewStore()
	for _, n := range []string{"a", "b", "c"} {
		if _, err := s.Create(n); err != nil {
			t.Fatalf("Create %q: %v", n, err)
		}
	}
	list := s.List()
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].ID >= list[i].ID {
			t.Errorf("List not sorted ascending by ID: %v", list)
		}
	}
}

func TestStoreListEmptyIsNonNil(t *testing.T) {
	s := NewStore()
	list := s.List()
	if list == nil {
		t.Fatalf("List() on empty store = nil, want non-nil empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("List() on empty store len = %d, want 0", len(list))
	}
}

// TestStoreConcurrentAccess exercises the s.mu mutex (store.go:20) by hammering
// every locking method from many goroutines at once. Run with -race to detect
// data races; without -race it still asserts no panic and a consistent count.
func TestStoreConcurrentAccess(t *testing.T) {
	s := NewStore()
	const workers = 50

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			// Each goroutine creates a uniquely named item, then reads.
			if _, err := s.Create("item-" + strconv.Itoa(n)); err != nil {
				t.Errorf("concurrent Create: %v", err)
			}
			_ = s.List()
			_, _ = s.Get(n)
		}(i)
	}
	wg.Wait()

	// Every Create used a distinct name, so all 50 must have persisted.
	if got := len(s.List()); got != workers {
		t.Errorf("after %d concurrent creates, List len = %d, want %d", workers, got, workers)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	it, _ := s.Create("temp")

	if err := s.Delete(it.ID); err != nil {
		t.Fatalf("Delete existing: %v", err)
	}
	if _, err := s.Get(it.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after Delete, Get err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(it.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}
