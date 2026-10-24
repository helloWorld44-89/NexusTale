package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(Config{Root: t.TempDir(), SigningKey: []byte("test-signing-key")})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPutOpenDelete(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "wiki/entities/abc/pic.png"

	if err := c.Put(ctx, key, strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	f, _, err := c.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(f)
	f.Close()
	if string(b) != "hello" {
		t.Fatalf("got %q", b)
	}

	// Overwrite replaces content atomically and leaves no temp files behind.
	if err := c.Put(ctx, key, strings.NewReader("world")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(filepath.Join(c.root, key)))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	if err := c.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Open(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// Empty parent dirs pruned, root preserved.
	if _, err := os.Stat(filepath.Join(c.root, "wiki")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected empty dirs pruned, stat err = %v", err)
	}
	if _, err := os.Stat(c.root); err != nil {
		t.Fatal("root must survive")
	}
	// Deleting a missing key is not an error.
	if err := c.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
}

func TestFailedPutKeepsExisting(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "exports/p/j.epub"
	if err := c.Put(ctx, key, strings.NewReader("good")); err != nil {
		t.Fatal(err)
	}
	if err := c.Put(ctx, key, io.MultiReader(strings.NewReader("par"), errReader{})); err == nil {
		t.Fatal("expected error")
	}
	f, _, err := c.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	if !bytes.Equal(b, []byte("good")) {
		t.Fatalf("existing object clobbered: %q", b)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestInvalidKeys(t *testing.T) {
	c := newTestClient(t)
	bad := []string{"", "../x", "a/../../x", "/abs", "a//b", "a/./b", "a/b/", "a\\b", "a/b c", "a/%2e%2e/b", strings.Repeat("a", 600)}
	for _, k := range bad {
		if _, err := c.pathFor(k); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("key %q should be invalid", k)
		}
		if err := c.Put(context.Background(), k, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q) should fail with ErrInvalidKey, got %v", k, err)
		}
	}
	if _, err := c.pathFor("wiki/entities/0f8-a/abc_1.png"); err != nil {
		t.Errorf("valid key rejected: %v", err)
	}
}

func TestSignedURL(t *testing.T) {
	c := newTestClient(t)
	key := "exports/p/j.epub"

	u, err := c.SignedURL(key, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, URLPrefix+key+"?") {
		t.Fatalf("unexpected url %s", u)
	}
	q := u[strings.Index(u, "?")+1:]
	var exp, sig string
	for _, kv := range strings.Split(q, "&") {
		p := strings.SplitN(kv, "=", 2)
		switch p[0] {
		case "exp":
			exp = p[1]
		case "sig":
			sig = p[1]
		}
	}
	if !c.VerifySignature(key, exp, sig) {
		t.Fatal("valid signature rejected")
	}
	if c.VerifySignature("exports/p/other.epub", exp, sig) {
		t.Fatal("signature must be bound to the key")
	}
	if c.VerifySignature(key, "9999999999", sig) {
		t.Fatal("signature must be bound to the expiry")
	}
	if c.VerifySignature(key, exp, "zz") || c.VerifySignature(key, "", "") {
		t.Fatal("garbage accepted")
	}

	other, _ := New(Config{Root: t.TempDir(), SigningKey: []byte("different-key")})
	if other.VerifySignature(key, exp, sig) {
		t.Fatal("signature valid under a different signing key")
	}

	expired, _ := c.SignedURL(key, -time.Minute)
	qs := expired[strings.Index(expired, "?")+1:]
	var e2, s2 string
	for _, kv := range strings.Split(qs, "&") {
		p := strings.SplitN(kv, "=", 2)
		if p[0] == "exp" {
			e2 = p[1]
		} else if p[0] == "sig" {
			s2 = p[1]
		}
	}
	if c.VerifySignature(key, e2, s2) {
		t.Fatal("expired signature accepted")
	}

	if _, err := c.SignedURL("../etc/passwd", time.Hour); !errors.Is(err, ErrInvalidKey) {
		t.Fatal("signing an invalid key must fail")
	}
}
