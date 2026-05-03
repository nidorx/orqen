package tinylfu

import (
	"container/list"
)

// slruCache implements a Segmented LRU cache with two segments:
// - "one" (probationary): newly added items
// - "two" (protected): items that have been accessed more than once
// Items move from "one" to "two" on second access.
// It is not safe for concurrent access.
type slruCache struct {
	data           map[string]*list.Element
	onecap, twocap int
	one, two       *list.List
}

func newSLRU(onecap, twocap int, data map[string]*list.Element) *slruCache {
	return &slruCache{
		data:   data,
		onecap: onecap,
		one:    list.New(),
		twocap: twocap,
		two:    list.New(),
	}
}

// get promotes an item: moves it to front if in "two", or moves it from "one" to "two".
func (slru *slruCache) get(v *list.Element) {
	item := v.Value.(*Item)

	// already on list two?
	if item.listid == 2 {
		slru.two.MoveToFront(v)
		return
	}

	// must be list one

	// is there space on the next list?
	if slru.two.Len() < slru.twocap {
		// just do the remove/add
		slru.one.Remove(v)
		item.listid = 2
		slru.data[item.Key] = slru.two.PushFront(item)
		return
	}

	back := slru.two.Back()
	bitem := back.Value.(*Item)

	// swap the key/values
	*bitem, *item = *item, *bitem

	bitem.listid = 2
	item.listid = 1

	// update pointers in the map
	slru.data[item.Key] = v
	slru.data[bitem.Key] = back

	// move the elements to the front of their lists
	slru.one.MoveToFront(v)
	slru.two.MoveToFront(back)
}

// add inserts a new item into the "one" (probationary) segment.
// If both segments are full, it evicts the LRU item from "one".
func (slru *slruCache) add(newItem *Item) {
	newItem.listid = 1

	if slru.one.Len() < slru.onecap || (slru.Len() < slru.onecap+slru.twocap) {
		slru.data[newItem.Key] = slru.one.PushFront(newItem)
		return
	}

	// reuse the tail item
	e := slru.one.Back()
	item := e.Value.(*Item)

	delete(slru.data, item.Key)

	*item = *newItem

	slru.data[item.Key] = e
	slru.one.MoveToFront(e)
}

// victim returns the LRU item in the "one" segment that would be evicted next.
// Returns nil if there is space available.
func (slru *slruCache) victim() *Item {
	if slru.Len() < slru.onecap+slru.twocap {
		return nil
	}

	v := slru.one.Back()

	return v.Value.(*Item)
}

// Len returns the total number of items in both segments.
func (slru *slruCache) Len() int {
	return slru.one.Len() + slru.two.Len()
}

// Remove removes an item from its respective segment.
func (slru *slruCache) Remove(v *list.Element) {
	item := v.Value.(*Item)
	if item.listid == 2 {
		slru.two.Remove(v)
	} else {
		slru.one.Remove(v)
	}
}
