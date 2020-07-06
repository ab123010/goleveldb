// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

// This a copy of Go std bytes.Buffer with some modification
// and some features stripped.
// 对GO标准bytes.Buffer进行改动

import (
	"bytes"
	"io"
)

// A Buffer is a variable-sized buffer of bytes with Read and Write methods.
// The zero value for Buffer is an empty buffer ready to use.
// 大小可变的、具有关于字节读写方法的buffer
// 零值的buff是一个空的用于读的buffer
type Buffer struct {
	buf       []byte   // contents are the bytes buf[off : len(buf)]	内容
	off       int      // read at &buf[off], write at &buf[len(buf)]	读buf[off]、写buf[len(buf)]的位置
	// 存放第一个切片的内存。 帮助小缓冲区（Printf）避免分配
	bootstrap [64]byte // memory to hold first slice; helps small buffers (Printf) avoid allocation.
}

// Bytes returns a slice of the contents of the unread portion of the buffer;
// len(b.Bytes()) == b.Len().  If the caller changes the contents of the
// returned slice, the contents of the buffer will change provided there
// are no intervening method calls on the Buffer.
// 返回缓冲区未读部分内容
// 如果调用者更改了返回的片的内容，则缓冲区的内容将发生变化，没有中间方法调用。
func (b *Buffer) Bytes() []byte { return b.buf[b.off:] }

// String returns the contents of the unread portion of the buffer
// as a string.  If the Buffer is a nil pointer, it returns "<nil>".
// 以string返回缓冲区未读部分内容
func (b *Buffer) String() string {
	if b == nil {
		// Special case, useful in debugging.
		return "<nil>"
	}
	return string(b.buf[b.off:])
}

// Len returns the number of bytes of the unread portion of the buffer;
// b.Len() == len(b.Bytes()).
// 返回缓冲区未读部分长度
func (b *Buffer) Len() int { return len(b.buf) - b.off }

// Truncate discards all but the first n unread bytes from the buffer.
// It panics if n is negative or greater than the length of the buffer.
// 丢弃缓冲区中除前n个未读取字节之外的所有字节，如果n为负或大于缓冲区长度，则会发生混乱。
func (b *Buffer) Truncate(n int) {
	switch {
	case n < 0 || n > b.Len():
		panic("leveldb/util.Buffer: truncation out of range")
	case n == 0:
		// Reuse buffer space.
		b.off = 0
	}
	b.buf = b.buf[0 : b.off+n]
}

// Reset resets the buffer so it has no content.
// b.Reset() is the same as b.Truncate(0).
// 重置缓冲区
func (b *Buffer) Reset() { b.Truncate(0) }

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with bytes.ErrTooLarge.
// 增加缓冲区以保证有n个字节的空间。返回应该在其中写入字节的索引。
func (b *Buffer) grow(n int) int {
	m := b.Len()
	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Truncate(0)
	}
	if len(b.buf)+n > cap(b.buf) {
		var buf []byte
		if b.buf == nil && n <= len(b.bootstrap) {
			// buf不存在，且n<len(b.bootstrap)
			buf = b.bootstrap[0:]
		} else if m+n <= cap(b.buf)/2 {
			// We can slide things down instead of allocating a new
			// slice. We only need m+n <= cap(b.buf) to slide, but
			// we instead let capacity get twice as large so we
			// don't spend all our time copying.
			// 未读长度+n < cap(b.buf)/2
			// 删除已读部分实现扩容
			copy(b.buf[:], b.buf[b.off:])
			buf = b.buf[:m]
		} else {
			// not enough space anywhere
			// 新建slice，容量*2
			buf = makeSlice(2*cap(b.buf) + n)
			copy(buf, b.buf[b.off:])
		}
		b.buf = buf
		b.off = 0
	}
	b.buf = b.buf[0 : b.off+m+n]
	return b.off + m
}

// Alloc allocs n bytes of slice from the buffer, growing the buffer as
// needed. If n is negative, Alloc will panic.
// If the buffer can't grow it will panic with bytes.ErrTooLarge.
// 从buffer分配一个n bytes大小的slice。返回b.buf，从写入index开始，保证其可用字节容量有n
func (b *Buffer) Alloc(n int) []byte {
	if n < 0 {
		panic("leveldb/util.Buffer.Alloc: negative count")
	}
	m := b.grow(n)
	return b.buf[m:]
}

// Grow grows the buffer's capacity, if necessary, to guarantee space for
// another n bytes. After Grow(n), at least n bytes can be written to the
// buffer without another allocation.
// If n is negative, Grow will panic.
// If the buffer can't grow it will panic with bytes.ErrTooLarge.
// 提高capacity。在Grow（n）之后，至少可以将n个字节写入缓冲区而无需进行其他分配。
func (b *Buffer) Grow(n int) {
	if n < 0 {
		panic("leveldb/util.Buffer.Grow: negative count")
	}
	m := b.grow(n)
	b.buf = b.buf[0:m]
}

// Write appends the contents of p to the buffer, growing the buffer as
// needed. The return value n is the length of p; err is always nil. If the
// buffer becomes too large, Write will panic with bytes.ErrTooLarge.
// 向buffer写入p
func (b *Buffer) Write(p []byte) (n int, err error) {
	// 保证容量足够
	m := b.grow(len(p))
	return copy(b.buf[m:], p), nil
}

// MinRead is the minimum slice size passed to a Read call by
// Buffer.ReadFrom.  As long as the Buffer has at least MinRead bytes beyond
// what is required to hold the contents of r, ReadFrom will not grow the
// underlying buffer.
// 对于Buffer.ReadFrom，buf的最小剩余容量
const MinRead = 512

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned. If the
// buffer becomes too large, ReadFrom will panic with bytes.ErrTooLarge.
// 从r读取数据直至EOF，并添加进buffer。返回值n为读取的字节数
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	// If buffer is empty, reset to recover space.
	if b.off >= len(b.buf) {
		b.Truncate(0)
	}
	for {
		// 维护剩余可用容量至少为MinRead
		if free := cap(b.buf) - len(b.buf); free < MinRead {
			// not enough space at end
			// 容量不够时，扩容，丢弃已读内容
			newBuf := b.buf
			if b.off+free < MinRead {
				// not enough space using beginning of buffer;
				// double buffer capacity
				newBuf = makeSlice(2*cap(b.buf) + MinRead)
			}
			copy(newBuf, b.buf[b.off:])
			b.buf = newBuf[:len(b.buf)-b.off]
			b.off = 0
		}
		m, e := r.Read(b.buf[len(b.buf):cap(b.buf)])
		b.buf = b.buf[0 : len(b.buf)+m]
		n += int64(m)
		if e == io.EOF {
			break
		}
		if e != nil {
			return n, e
		}
	}
	return n, nil // err is EOF, so return nil explicitly
}

// makeSlice allocates a slice of size n. If the allocation fails, it panics
// with bytes.ErrTooLarge.
func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(bytes.ErrTooLarge)
		}
	}()
	return make([]byte, n)
}

// WriteTo writes data to w until the buffer is drained or an error occurs.
// The return value n is the number of bytes written; it always fits into an
// int, but it is int64 to match the io.WriterTo interface. Any error
// encountered during the write is also returned.
// 将数据写入w，直到缓冲区耗尽或发生错误。返回写入多少字节n
func (b *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	if b.off < len(b.buf) {
		nBytes := b.Len()
		m, e := w.Write(b.buf[b.off:])
		if m > nBytes {
			panic("leveldb/util.Buffer.WriteTo: invalid Write count")
		}
		b.off += m
		n = int64(m)
		if e != nil {
			return n, e
		}
		// all bytes should have been written, by definition of
		// Write method in io.Writer
		if m != nBytes {
			return n, io.ErrShortWrite
		}
	}
	// Buffer is now empty; reset.
	b.Truncate(0)
	return
}

// WriteByte appends the byte c to the buffer, growing the buffer as needed.
// The returned error is always nil, but is included to match bufio.Writer's
// WriteByte. If the buffer becomes too large, WriteByte will panic with
// bytes.ErrTooLarge.
// 将字节c附加到buffer，根据需要增大缓冲区。
func (b *Buffer) WriteByte(c byte) error {
	m := b.grow(1)
	b.buf[m] = c
	return nil
}

// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained.  The return value n is the number of bytes read.  If the
// buffer has no data to return, err is io.EOF (unless len(p) is zero);
// otherwise it is nil.
// 从缓冲区读取下一个len（p）字节，或者直到缓冲区耗尽为止。
func (b *Buffer) Read(p []byte) (n int, err error) {
	if b.off >= len(b.buf) {
		// Buffer is empty, reset to recover space.
		// buffer内容全部读出，重置buffer恢复空间
		b.Truncate(0)
		if len(p) == 0 {
			return
		}
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.off:])
	b.off += n
	return
}

// Next returns a slice containing the next n bytes from the buffer,
// advancing the buffer as if the bytes had been returned by Read.
// If there are fewer than n bytes in the buffer, Next returns the entire buffer.
// The slice is only valid until the next call to a read or write method.
// Next返回一个包含缓冲区中接下来的n个字节的切片，使缓冲区前进，就好像该字节已由Read读出。
func (b *Buffer) Next(n int) []byte {
	m := b.Len()
	if n > m {
		n = m
	}
	data := b.buf[b.off : b.off+n]
	b.off += n
	return data
}

// ReadByte reads and returns the next byte from the buffer.
// If no byte is available, it returns error io.EOF.
// 读取并返回buffer下一字节
func (b *Buffer) ReadByte() (c byte, err error) {
	if b.off >= len(b.buf) {
		// Buffer is empty, reset to recover space.
		b.Truncate(0)
		return 0, io.EOF
	}
	c = b.buf[b.off]
	b.off++
	return c, nil
}

// ReadBytes reads until the first occurrence of delim in the input,
// returning a slice containing the data up to and including the delimiter.
// If ReadBytes encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadBytes returns err != nil if and only if the returned data does not end in
// delim.
// 读取直到输入中第一次出现delim为止，返回包含数据的切片。
func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	slice, err := b.readSlice(delim)
	// return a copy of slice. The buffer's backing array may
	// be overwritten by later calls.
	line = append(line, slice...)
	return
}

// readSlice is like ReadBytes but returns a reference to internal buffer data.
// 返回未读内容，到第一次出现delim截止
func (b *Buffer) readSlice(delim byte) (line []byte, err error) {
	i := bytes.IndexByte(b.buf[b.off:], delim)
	end := b.off + i + 1
	if i < 0 {
		end = len(b.buf)
		err = io.EOF
	}
	line = b.buf[b.off:end]
	b.off = end
	return line, err
}

// NewBuffer creates and initializes a new Buffer using buf as its initial
// contents.  It is intended to prepare a Buffer to read existing data.  It
// can also be used to size the internal buffer for writing. To do that,
// buf should have the desired capacity but a length of zero.
//
// In most cases, new(Buffer) (or just declaring a Buffer variable) is
// sufficient to initialize a Buffer.
func NewBuffer(buf []byte) *Buffer { return &Buffer{buf: buf} }
