// Package storage keeps binary assets (wiki entity images, exported EPUB/DOCX
// files) on the local filesystem under a single root directory.
//
// Access control model: objects are never served by path alone. Callers hand
// the browser a short-lived HMAC-signed URL (SignedURL); the files route
// verifies it (VerifySignature) before opening anything. This lets <img src>
// and plain download links work without an Authorization header, which is the
// same property the old MinIO presigned URLs gave us.
package storage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// URLPrefix is the public route the signed URLs point at. It must match the
// route registered by the files handler.
const URLPrefix = "/api/v1/files/"

// signingDomain separates these signatures from any other use of the same
// secret (e.g. JWTs), so a signature minted here can never be valid elsewhere.
const signingDomain = "nexustale-file-v1\n"

// ErrNotFound is returned by Open when the object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// ErrInvalidKey is returned for keys that could escape the storage root or
// are otherwise malformed.
var ErrInvalidKey = errors.New("storage: invalid key")

// Config configures the local-disk store.
type Config struct {
	// Root is the directory objects live under; created if missing.
	Root string
	// SigningKey authenticates download URLs. Must be non-empty.
	SigningKey []byte
}

// Client is a local-disk object store. Safe for concurrent use.
type Client struct {
	root       string
	signingKey []byte
}

// New creates the root directory (if needed) and returns a Client.
func New(cfg Config) (*Client, error) {
	if cfg.Root == "" {
		return nil, errors.New("storage: root path is required")
	}
	if len(cfg.SigningKey) == 0 {
		return nil, errors.New("storage: signing key is required")
	}
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve root: %w", err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("storage: create root: %w", err)
	}
	return &Client{root: root, signingKey: cfg.SigningKey}, nil
}

// pathFor maps a key to an absolute path inside root.
//
// Keys are server-generated ("exports/<uuid>/<uuid>.epub"), but the files
// route also receives them from the network, so this is the single choke
// point that guarantees a key cannot leave root: every segment must be a
// plain name made of [A-Za-z0-9._-], and "." / ".." are rejected outright.
func (c *Client) pathFor(key string) (string, error) {
	if key == "" || len(key) > 512 {
		return "", ErrInvalidKey
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", ErrInvalidKey
		}
		for _, r := range seg {
			ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
			if !ok {
				return "", ErrInvalidKey
			}
		}
	}
	return filepath.Join(c.root, filepath.FromSlash(key)), nil
}

// Put writes r to key atomically: data goes to a temp file in the destination
// directory and is renamed into place, so readers never observe a partial
// file and a failed upload never clobbers an existing object.
func (c *Client) Put(ctx context.Context, key string, r io.Reader) error {
	p, err := c.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("storage: mkdir for %s: %w", key, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return fmt.Errorf("storage: temp file for %s: %w", key, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := io.Copy(tmp, &ctxReader{ctx: ctx, r: r}); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("storage: write %s: %w", key, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("storage: close %s: %w", key, err)
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		cleanup()
		return fmt.Errorf("storage: chmod %s: %w", key, err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		cleanup()
		return fmt.Errorf("storage: commit %s: %w", key, err)
	}
	return nil
}

// Open returns the object's file handle and modification time. The caller
// closes it. Returns ErrNotFound if the object is absent.
func (c *Client) Open(key string) (*os.File, time.Time, error) {
	p, err := c.pathFor(key)
	if err != nil {
		return nil, time.Time{}, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, time.Time{}, ErrNotFound
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("storage: open %s: %w", key, err)
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		_ = f.Close()
		return nil, time.Time{}, ErrNotFound
	}
	return f, info.ModTime(), nil
}

// Delete removes the object. A missing key is not an error. Empty parent
// directories are pruned (best effort) so per-entity / per-project folders
// don't accumulate.
func (c *Client) Delete(_ context.Context, key string) error {
	p, err := c.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	// os.Remove on a directory fails unless it's empty, which is exactly the
	// condition we want; stop at the first failure or at root.
	for dir := filepath.Dir(p); dir != c.root && strings.HasPrefix(dir, c.root); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// SignedURL returns a relative, time-limited download URL for key.
func (c *Client) SignedURL(key string, expiry time.Duration) (string, error) {
	if _, err := c.pathFor(key); err != nil {
		return "", err
	}
	exp := time.Now().Add(expiry).Unix()
	q := url.Values{}
	q.Set("exp", strconv.FormatInt(exp, 10))
	q.Set("sig", c.sign(key, exp))
	return URLPrefix + key + "?" + q.Encode(), nil
}

// VerifySignature reports whether sig is a valid, unexpired signature for key.
func (c *Client) VerifySignature(key, expStr, sig string) bool {
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	want, err := hex.DecodeString(c.sign(key, exp))
	if err != nil {
		return false
	}
	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	return hmac.Equal(want, got)
}

func (c *Client) sign(key string, exp int64) string {
	m := hmac.New(sha256.New, c.signingKey)
	m.Write([]byte(signingDomain + key + "\n" + strconv.FormatInt(exp, 10)))
	return hex.EncodeToString(m.Sum(nil))
}

// ContentTypeFor returns the MIME type for a key based on its extension.
// Types are an explicit allowlist rather than mime.TypeByExtension, which
// depends on system mime files that minimal container images don't ship.
func ContentTypeFor(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".epub":
		return "application/epub+zip"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}

// RandomName returns a 128-bit random hex string, for building object keys
// that never collide and never carry user-controlled text.
func RandomName() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("storage: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// ctxReader aborts a copy when the request context is cancelled.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
