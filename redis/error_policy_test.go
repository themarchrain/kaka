package redis

import (
	"context"
	"testing"
	"time"
)

// 用已关闭的连接制造底层错误，验证 Allow 按 ErrorPolicy 降级。
func TestTokenBucketFailClosedOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close() // 制造网络错误

	var reported error
	tb := NewTokenBucket(client, 1, 1,
		WithOnError(func(err error) { reported = err }))

	res, err := tb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure (policy downgrade): %v", err)
	}
	if res.Allowed {
		t.Fatal("fail-closed: should be denied")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}

func TestTokenBucketFailOpenOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close()

	var reported error
	tb := NewTokenBucket(client, 1, 1,
		WithErrorPolicy(ErrorFailOpen),
		WithOnError(func(err error) { reported = err }))

	res, err := tb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure (policy downgrade): %v", err)
	}
	if !res.Allowed {
		t.Fatal("fail-open: should be allowed")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}

func TestLeakyBucketFailClosedOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close()

	var reported error
	lb := NewLeakyBucket(client, 1, 1,
		WithOnError(func(err error) { reported = err }))

	res, err := lb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure: %v", err)
	}
	if res.Allowed {
		t.Fatal("fail-closed: should be denied")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}

func TestLeakyBucketFailOpenOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close()

	var reported error
	lb := NewLeakyBucket(client, 1, 1,
		WithErrorPolicy(ErrorFailOpen),
		WithOnError(func(err error) { reported = err }))

	res, err := lb.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure: %v", err)
	}
	if !res.Allowed {
		t.Fatal("fail-open: should be allowed")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}

func TestSlidingWindowFailClosedOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close()

	var reported error
	sw := NewSlidingWindow(client, 1, time.Minute,
		WithOnError(func(err error) { reported = err }))

	res, err := sw.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure: %v", err)
	}
	if res.Allowed {
		t.Fatal("fail-closed: should be denied")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}

func TestSlidingWindowFailOpenOnDeadClient(t *testing.T) {
	client := testClient(t)
	client.Close()

	var reported error
	sw := NewSlidingWindow(client, 1, time.Minute,
		WithErrorPolicy(ErrorFailOpen),
		WithOnError(func(err error) { reported = err }))

	res, err := sw.Allow(context.Background(), "k1")
	if err != nil {
		t.Fatalf("Allow should not return error on redis failure: %v", err)
	}
	if !res.Allowed {
		t.Fatal("fail-open: should be allowed")
	}
	if reported == nil {
		t.Fatal("onError should have been called")
	}
}
