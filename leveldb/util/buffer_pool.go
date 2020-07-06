// Copyright (c) 2014, Suryandaru Triandana <syndtr@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type buffer struct {
	b    []byte
	miss int
}

// BufferPool is a 'buffer pool'.
type BufferPool struct {
	pool      [6]chan []byte
	size      [5]uint32
	sizeMiss  [5]uint32
	sizeHalf  [5]uint32
	baseline  [4]int		// {baseline / 4, baseline / 2, baseline * 2, baseline * 4}
	baseline0 int			// baseline

	mu     sync.RWMutex
	closed bool				// 关闭标志
	closeC chan struct{}	// 通知关闭信号

	get     uint32
	put     uint32
	half    uint32
	less    uint32
	equal   uint32
	greater uint32
	miss    uint32
}

// 判断buffer放入哪个通道
func (p *BufferPool) poolNum(n int) int {
	// 大于baseline0/2，且小于baseline0，放入0号
	if n <= p.baseline0 && n > p.baseline0/2 {
		return 0
	}
	// 根据baseline容量标准判断
	for i, x := range p.baseline {
		if n <= x {
			return i + 1
		}
	}
	// 大于baseline * 4，放入5号
	return len(p.baseline) + 1
}

// Get returns buffer with length of n.
func (p *BufferPool) Get(n int) []byte {
	// pool为空
	if p == nil {
		return make([]byte, n)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		// poor关闭
		return make([]byte, n)
	}

	atomic.AddUint32(&p.get, 1)

	poolNum := p.poolNum(n)
	pool := p.pool[poolNum]
	if poolNum == 0 {
		// Fast path.
		// 返回一个[]byte，并根据pool中[]byte和实际需求，增加pool相应状态值
		select {
		case b := <-pool:
			switch {
			case cap(b) > n:
				if cap(b)-n >= n {
					// 容量为n2倍多，新建[]byte返回，将b重新放回pool
					atomic.AddUint32(&p.half, 1)
					select {
					case pool <- b:
					default:
					}
					return make([]byte, n)
				} else {
					atomic.AddUint32(&p.less, 1)
					return b[:n]
				}
			case cap(b) == n:
				atomic.AddUint32(&p.equal, 1)
				return b[:n]
			default:
				atomic.AddUint32(&p.greater, 1)
			}
		default:
			atomic.AddUint32(&p.miss, 1)
		}

		// 容量小于或pool空，新建[]byte返回
		return make([]byte, n, p.baseline0)
	} else {
		sizePtr := &p.size[poolNum-1]

		select {
		case b := <-pool:
			switch {
			case cap(b) > n:
				if cap(b)-n >= n {
					// 容量为n2倍多，新建[]byte返回，b放回pool
					atomic.AddUint32(&p.half, 1)
					sizeHalfPtr := &p.sizeHalf[poolNum-1]
					if atomic.AddUint32(sizeHalfPtr, 1) == 20 {
						// 此情景发生次数达20时，b丢弃，在size[]对应位置记录(cap(b)/2)，重置计数
						atomic.StoreUint32(sizePtr, uint32(cap(b)/2))
						atomic.StoreUint32(sizeHalfPtr, 0)
					} else {
						select {
						case pool <- b:
						default:
						}
					}
					return make([]byte, n)
				} else {
					atomic.AddUint32(&p.less, 1)
					return b[:n]
				}
			case cap(b) == n:
				atomic.AddUint32(&p.equal, 1)
				return b[:n]
			default:
				atomic.AddUint32(&p.greater, 1)
				if uint32(cap(b)) >= atomic.LoadUint32(sizePtr) {
					// 容量小于n时，若b容量大于size[]对应位置记录，将b放回pool
					select {
					case pool <- b:
					default:
					}
				}
			}
		default:
			atomic.AddUint32(&p.miss, 1)
		}

		if size := atomic.LoadUint32(sizePtr); uint32(n) > size {
			// size小于n时，返回大小为n切片
			if size == 0 {
				// []size对应位置存uint32(n)
				atomic.CompareAndSwapUint32(sizePtr, 0, uint32(n))
			} else {
				sizeMissPtr := &p.sizeMiss[poolNum-1]
				// 未找到记录+1
				if atomic.AddUint32(sizeMissPtr, 1) == 20 {
					// 未找到20次时，[]size对应位置放入uint32(n)，重置计数
					atomic.StoreUint32(sizePtr, uint32(n))
					atomic.StoreUint32(sizeMissPtr, 0)
				}
			}
			return make([]byte, n)
		} else {
			// size大于n时，返回大小为n容量为size切片
			return make([]byte, n, size)
		}
	}
}

// Put adds given buffer to the pool.
func (p *BufferPool) Put(b []byte) {
	if p == nil {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return
	}

	atomic.AddUint32(&p.put, 1)

	// 根据b容量将其放入pool
	pool := p.pool[p.poolNum(cap(b))]
	select {
	case pool <- b:
	default:
	}

}

func (p *BufferPool) Close() {
	if p == nil {
		return
	}

	p.mu.Lock()
	if !p.closed {
		p.closed = true
		p.closeC <- struct{}{}
	}
	p.mu.Unlock()
}

// 获取pool信息
func (p *BufferPool) String() string {
	if p == nil {
		return "<nil>"
	}

	return fmt.Sprintf("BufferPool{B·%d Z·%v Zm·%v Zh·%v G·%d P·%d H·%d <·%d =·%d >·%d M·%d}",
		p.baseline0, p.size, p.sizeMiss, p.sizeHalf, p.get, p.put, p.half, p.less, p.equal, p.greater, p.miss)
}

func (p *BufferPool) drain() {
	ticker := time.NewTicker(2 * time.Second)	// 每隔一段时间发信号
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, ch := range p.pool {
				select {
				case <-ch:
				default:
				}
			}
		case <-p.closeC:
			close(p.closeC)
			// 关闭pool所有chan
			for _, ch := range p.pool {
				close(ch)
			}
			return
		}
	}
}

// NewBufferPool creates a new initialized 'buffer pool'.
func NewBufferPool(baseline int) *BufferPool {
	if baseline <= 0 {
		panic("baseline can't be <= 0")
	}
	p := &BufferPool{
		baseline0: baseline,
		baseline:  [...]int{baseline / 4, baseline / 2, baseline * 2, baseline * 4},
		closeC:    make(chan struct{}, 1),
	}
	for i, cap := range []int{2, 2, 4, 4, 2, 1} {
		p.pool[i] = make(chan []byte, cap)
	}
	go p.drain()
	return p
}
