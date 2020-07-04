// Copyright (c) 2014, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iterator

import (
	"github.com/syndtr/goleveldb/leveldb/util"
)

// BasicArray is the interface that wraps basic Len and Search method.
type BasicArray interface {
	// Len returns length of the array.
	Len() int

	// Search finds smallest index that point to a key that is greater
	// than or equal to the given key.
	// 找到指向大于或等于给定key的key的最小索引。
	Search(key []byte) int
}

// Array is the interface that wraps BasicArray and basic Index method.
type Array interface {
	BasicArray

	// Index returns key/value pair with index of i.
	// 返回索引为i的键值对
	Index(i int) (key, value []byte)
}

// ArrayIndexer is the interface that wraps BasicArray and basic Get method.
type ArrayIndexer interface {
	BasicArray

	// Get returns a new data iterator with index of i.
	// 用索引i获取一个新的数据迭代器
	Get(i int) Iterator
}

type basicArrayIterator struct {
	util.BasicReleaser
	array BasicArray
	pos   int			// 记录目前遍历位置
	err   error
}

func (i *basicArrayIterator) Valid() bool {
	// 验证可用性，目前遍历位置合法，且迭代器未被释放
	return i.pos >= 0 && i.pos < i.array.Len() && !i.Released()
}

func (i *basicArrayIterator) First() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	if i.array.Len() == 0 {
		i.pos = -1
		return false
	}
	i.pos = 0
	return true
}

func (i *basicArrayIterator) Last() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	n := i.array.Len()
	if n == 0 {
		i.pos = 0
		return false
	}
	i.pos = n - 1
	return true
}

func (i *basicArrayIterator) Seek(key []byte) bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	n := i.array.Len()
	if n == 0 {
		i.pos = 0
		return false
	}
	i.pos = i.array.Search(key)
	if i.pos >= n {
		return false
	}
	return true
}

// pos++，若已迭代到最后位置，pos不增加，返回false
func (i *basicArrayIterator) Next() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	i.pos++
	if n := i.array.Len(); i.pos >= n {
		i.pos = n
		return false
	}
	return true
}

// pos--，若pos位于0位置，置为-1，返回false
func (i *basicArrayIterator) Prev() bool {
	if i.Released() {
		i.err = ErrIterReleased
		return false
	}

	i.pos--
	if i.pos < 0 {
		i.pos = -1
		return false
	}
	return true
}

func (i *basicArrayIterator) Error() error { return i.err }

type arrayIterator struct {
	basicArrayIterator
	array      Array
	pos        int
	key, value []byte
}

// 更新目前arrayIterator中所存键值对，为数据遍历位置的键值对
func (i *arrayIterator) updateKV() {
	if i.pos == i.basicArrayIterator.pos {
		// 此时保存的键值对即为所求
		return
	}
	i.pos = i.basicArrayIterator.pos
	if i.Valid() {
		// 获取所求索引未知的键值对
		i.key, i.value = i.array.Index(i.pos)
	} else {
		i.key = nil
		i.value = nil
	}
}

func (i *arrayIterator) Key() []byte {
	i.updateKV()
	return i.key
}

func (i *arrayIterator) Value() []byte {
	i.updateKV()
	return i.value
}

type arrayIteratorIndexer struct {
	basicArrayIterator
	array ArrayIndexer
}

func (i *arrayIteratorIndexer) Get() Iterator {
	if i.Valid() {
		return i.array.Get(i.basicArrayIterator.pos)
	}
	return nil
}

// NewArrayIterator returns an iterator from the given array.
func NewArrayIterator(array Array) Iterator {
	return &arrayIterator{
		basicArrayIterator: basicArrayIterator{array: array, pos: -1},
		array:              array,
		pos:                -1,
	}
}

// NewArrayIndexer returns an index iterator from the given array.
// 可使用索引i获取array的迭代器
func NewArrayIndexer(array ArrayIndexer) IteratorIndexer {
	return &arrayIteratorIndexer{
		basicArrayIterator: basicArrayIterator{array: array, pos: -1},
		array:              array,
	}
}
