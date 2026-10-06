package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeviceCode(t *testing.T) {
	s, ctx := New(testDB(t)), context.Background()
	u, _ := s.CreateUser(ctx, "a", "h", false)
	if err := s.CreateDeviceCode(ctx, "ABCDEFGH", "tok", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDeviceCode(ctx, "OLDCODE1", "old", time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}

	if id, err := s.ClaimDeviceCode(ctx, "tok"); err != nil || id != 0 {
		t.Fatalf("pending claim = %d, %v", id, err)
	}
	if err := s.ApproveDeviceCode(ctx, "ABCDEFGH", u.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ApproveDeviceCode(ctx, "ABCDEFGH", u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second approval = %v, want ErrNotFound", err)
	}
	if id, err := s.ClaimDeviceCode(ctx, "tok"); err != nil || id != u.ID {
		t.Fatalf("approved claim = %d, %v", id, err)
	}
	if _, err := s.ClaimDeviceCode(ctx, "tok"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim after use = %v, want ErrNotFound", err)
	}
	if err := s.ApproveDeviceCode(ctx, "OLDCODE1", u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired approval = %v, want ErrNotFound", err)
	}
	if _, err := s.ClaimDeviceCode(ctx, "unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown claim = %v, want ErrNotFound", err)
	}
}
