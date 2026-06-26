package seeds

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store errors.
var (
	ErrNotFound  = errors.New("item not found")
	ErrDuplicate = errors.New("an item with that name already exists")
)

// Store is a thread-safe in-memory item store. IDs are assigned sequentially
// starting at 1. Names are unique case-insensitively.
type Store struct {
	mu     sync.Mutex
	items  map[int]Item
	nextID int
	now    func() time.Time
}

// NewStore returns an empty Store using the wall clock for timestamps.
func NewStore() *Store {
	return &Store{items: map[int]Item{}, nextID: 1, now: time.Now}
}

// Create validates name, rejects a case-insensitive duplicate, and inserts a
// new item. The stored name is the trimmed form of the input.
func (s *Store) Create(name string) (Item, error) {
	if err := ValidateName(name); err != nil {
		return Item{}, err
	}
	trimmed := strings.TrimSpace(name)

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range s.items {
		if strings.EqualFold(it.Name, trimmed) {
			return Item{}, ErrDuplicate
		}
	}
	it := Item{ID: s.nextID, Name: trimmed, CreatedAt: s.now()}
	s.items[it.ID] = it
	s.nextID++
	return it, nil
}

// Get returns the item with the given id, or ErrNotFound.
func (s *Store) Get(id int) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return it, nil
}

// List returns all items sorted by ID ascending (creation order).
func (s *Store) List() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Delete removes the item with the given id, or returns ErrNotFound.
func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}
