// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package iterator provides interface and implementation to traverse over
// contents of a database.
package iterator

import (
	"errors"

	"github.com/syndtr/goleveldb/leveldb/util"
)

var (
	ErrIterReleased = errors.New("leveldb/iterator: iterator released")
)

// IteratorSeeker is the interface that wraps the 'seeks method'.
// 封装了寻找方法的接口
type IteratorSeeker interface {
	// First moves the iterator to the first key/value pair. If the iterator
	// only contains one key/value pair then First and Last would moves
	// to the same key/value pair.
	// It returns whether such pair exist.
	// 将迭代器移动到第一个键值对
	// 返回这样的键值对对是否存在
	First() bool

	// Last moves the iterator to the last key/value pair. If the iterator
	// only contains one key/value pair then First and Last would moves
	// to the same key/value pair.
	// It returns whether such pair exist.
	// 将迭代器移动到最后一个键值对
	// 返回这样的键值对对是否存在
	Last() bool

	// Seek moves the iterator to the first key/value pair whose key is greater
	// than or equal to the given key.
	// It returns whether such pair exist.
	//
	// It is safe to modify the contents of the argument after Seek returns.
	// 将迭代器移动到第一个大于等于给定key的键值对位置
	// 返回这样的键值对是否存在
	// 在Seek返回后，可以安全地修改参数的内容。
	Seek(key []byte) bool

	// Next moves the iterator to the next key/value pair.
	// It returns false if the iterator is exhausted.
	// 将迭代器移动到下一个键值对
	// 如果迭代器耗尽返回false
	Next() bool

	// Prev moves the iterator to the previous key/value pair.
	// It returns false if the iterator is exhausted.
	// 将迭代器移动到前一个键值对
	// 如果迭代器耗尽返回false
	Prev() bool
}

// CommonIterator is the interface that wraps common iterator methods.
// 封装公共迭代器方法的接口
type CommonIterator interface {
	IteratorSeeker

	// util.Releaser is the interface that wraps basic Release method.
	// When called Release will releases any resources associated with the
	// iterator.
	// 封装基本Release方法
	// 调用时释放迭代器的所有相关资源
	util.Releaser

	// util.ReleaseSetter is the interface that wraps the basic SetReleaser
	// method.
	// 封装SetReleaser方法
	util.ReleaseSetter

	// TODO: Remove this when ready.
	Valid() bool

	// Error returns any accumulated error. Exhausting all the key/value pairs
	// is not considered to be an error.
	Error() error
}

// Iterator iterates over a DB's key/value pairs in key order.
//
// When encounter an error any 'seeks method' will return false and will
// yield no key/value pairs. The error can be queried by calling the Error
// method. Calling Release is still necessary.
//
// An iterator must be released after use, but it is not necessary to read
// an iterator until exhaustion.
// Also, an iterator is not necessarily safe for concurrent use, but it is
// safe to use multiple iterators concurrently, with each in a dedicated
// goroutine.
// 按key顺序遍历键值对
// 遇到错误时seek方法会返回false并不产生键值对
// 迭代器使用完成后必须将其释放
// 同时一个迭代器使用时不安全的，但在各个goroutine中分别使用不同的迭代器是安全的
type Iterator interface {
	CommonIterator

	// Key returns the key of the current key/value pair, or nil if done.
	// The caller should not modify the contents of the returned slice, and
	// its contents may change on the next call to any 'seeks method'.
	// 返回目前位置键值对的key，迭代完成返回nil
	// 调用者不能修改返回的slice
	Key() []byte

	// Value returns the value of the current key/value pair, or nil if done.
	// The caller should not modify the contents of the returned slice, and
	// its contents may change on the next call to any 'seeks method'.
	// 返回目前位置键值对的value，迭代完成返回nil
	// 调用者不能修改返回的slice
	Value() []byte
}

// ErrorCallbackSetter is the interface that wraps basic SetErrorCallback
// method.
//
// ErrorCallbackSetter implemented by indexed and merged iterator.
type ErrorCallbackSetter interface {
	// SetErrorCallback allows set an error callback of the corresponding
	// iterator. Use nil to clear the callback.
	// 设置相应迭代器的错误回调，使用nil清除回调
	SetErrorCallback(f func(err error))
}

type emptyIterator struct {
	util.BasicReleaser
	err error
}

func (i *emptyIterator) rErr() {
	if i.err == nil && i.Released() {
		// i被释放，且无异常信息
		i.err = ErrIterReleased		// 设异常信息为迭代器已被释放
	}
}

func (*emptyIterator) Valid() bool            { return false }
func (i *emptyIterator) First() bool          { i.rErr(); return false }
func (i *emptyIterator) Last() bool           { i.rErr(); return false }
func (i *emptyIterator) Seek(key []byte) bool { i.rErr(); return false }
func (i *emptyIterator) Next() bool           { i.rErr(); return false }
func (i *emptyIterator) Prev() bool           { i.rErr(); return false }
func (*emptyIterator) Key() []byte            { return nil }
func (*emptyIterator) Value() []byte          { return nil }
func (i *emptyIterator) Error() error         { return i.err }

// NewEmptyIterator creates an empty iterator. The err parameter can be
// nil, but if not nil the given err will be returned by Error method.
func NewEmptyIterator(err error) Iterator {
	return &emptyIterator{err: err}
}
