package algorithm

import (
	"sync"
	"time"
)

type LeakyBucket struct {
	mu       sync.Mutex
	capacity float64   // 桶的容量
	rate     float64   // 漏水速率 (滴/秒)
	water    float64   // 当前桶里的水量
	lastLeak time.Time // 上次漏水的时间
}

func NewLeakyBucket(capacity, rate float64) *LeakyBucket {
	return &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		water:    0, // 初始空桶
		lastLeak: time.Now(),
	}
}

func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := time.Now()
	// 计算过去这段时间漏掉了多少水
	elapsed := now.Sub(lb.lastLeak).Seconds()
	leakedWater := elapsed * lb.rate

	// 扣除漏掉的水，水量不能小于0
	lb.water -= leakedWater
	if lb.water < 0 {
		lb.water = 0
	}
	lb.lastLeak = now

	// 尝试加入一滴新水：一个新请求
	// 如果当前水量 + 1滴水 没有超过容量，则允许
	if lb.water+1 <= lb.capacity {
		lb.water += 1
		return true
	}
	// 桶满了，溢出拒绝
	return false
}
