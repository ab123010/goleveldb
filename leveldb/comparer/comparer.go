// Copyright (c) 2012, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package comparer provides interface and implementation for ordering
// sets of data.
// 提供用于数据集排序的接口和其实现
package comparer

// BasicComparer is the interface that wraps the basic Compare method.
// 包装基本比较方法的接口
type BasicComparer interface {
	// Compare returns -1, 0, or +1 depending on whether a is 'less than',
	// 'equal to' or 'greater than' b. The two arguments can only be 'equal'
	// if their contents are exactly equal. Furthermore, the empty slice
	// must be 'less than' any non-empty slice.
	// 返回1，-1，0表示大小。空的slice小于任何非空slice
	Compare(a, b []byte) int
}

// Comparer defines a total ordering over the space of []byte keys: a 'less
// than' relationship.
type Comparer interface {
	BasicComparer

	// Name returns name of the comparer.
	//
	// The Level-DB on-disk format stores the comparer name, and opening a
	// database with a different comparer from the one it was created with
	// will result in an error.
	//
	// An implementation to a new name whenever the comparer implementation
	// changes in a way that will cause the relative ordering of any two keys
	// to change.
	//
	// Names starting with "leveldb." are reserved and should not be used
	// by any users of this package.
	//
	// 返回比较器名字
	// 磁盘格式存储比较器名字，需使用与数据库创建的比较器一样的比较器打开数据库
	Name() string

	// Bellow are advanced functions used to reduce the space requirements
	// for internal data structures such as index blocks.

	// Separator appends a sequence of bytes x to dst such that a <= x && x < b,
	// where 'less than' is consistent with Compare. An implementation should
	// return nil if x equal to a.
	//
	// Either contents of a or b should not by any means modified. Doing so
	// may cause corruption on the internal state.
	// 用于减少内部数据结构（例如索引块）的空间
	// 向dst中添加一个比特序列x，a <= x && x < b，x==a返回nil
	// a或b的内容不能通过任何方式修改，否则会导致内部状态损坏
	Separator(dst, a, b []byte) []byte

	// Successor appends a sequence of bytes x to dst such that x >= b, where
	// 'less than' is consistent with Compare. An implementation should return
	// nil if x equal to b.
	//
	// Contents of b should not by any means modified. Doing so may cause
	// corruption on the internal state.
	// 向dst中添加一个比特序列x，x >= b，x==b返回nil
	Successor(dst, b []byte) []byte
}
