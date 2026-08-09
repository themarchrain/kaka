package redis

import (
	"context"
	"testing"
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
