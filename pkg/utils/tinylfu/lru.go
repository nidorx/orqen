package tinylfu

import "container/list"

// lruCache is a simple LRU cache backed by a doubly-linked list.
// It is not safe for concurrent access.
type lruCache struct {
	data map[string]*list.Element
	cap  int
	ll   *list.List
}

func newLRU(cap int, data map[string]*list.Element) *lruCache {
	return &lruCache{
		data: data,
		cap:  cap,
		ll:   list.New(),
	}
}

// get moves an existing element to the front (most recently used).
func (lru *lruCache) get(v *list.Element) {
	lru.ll.MoveToFront(v)
}

// add inserts a new item into the LRU cache.
// If the cache is full, it evicts the least recently used item and returns it.
// Returns the evicted item and true if an eviction occurred.
func (lru *lruCache) add(newItem *Item) (_ *Item, evicted bool) {
	if lru.ll.Len() < lru.cap {
		lru.data[newItem.Key] = lru.ll.PushFront(newItem)
		return nil, false
	}

	// reuse the tail item (least recently used)
	val := lru.ll.Back()
	item := val.Value.(*Item)

	delete(lru.data, item.Key)

	oldItem := *item
	*item = *newItem

	lru.data[item.Key] = val
	lru.ll.MoveToFront(val)

	return &oldItem, true
}

// Len returns the number of items currently in the LRU cache.
func (lru *lruCache) Len() int {
	return lru.ll.Len()
}

// Remove removes an item from the underlying linked list.
func (lru *lruCache) Remove(v *list.Element) {
	lru.ll.Remove(v)
}
