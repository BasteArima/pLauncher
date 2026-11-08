package parser

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding/charmap"
)

type pornolabCachedPage struct {
	data []byte
	used time.Time
}
type pornolabFlight struct {
	done chan struct{}
	data []byte
	err  error
}
type pornolabGenerationKey struct{}
type pornolabGeneration struct {
	cache *PornolabCache
	epoch uint64
}

// Zero value is ready to use. At most five raw pages (4 MiB each); DOMs are never shared.
type PornolabCache struct {
	mu      sync.Mutex
	pages   map[string]pornolabCachedPage
	flights map[string]*pornolabFlight
	epoch   uint64
	ctx     context.Context
	cancel  context.CancelFunc
	now     func() time.Time
}

func (c *PornolabCache) initLocked() {
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	if c.pages == nil {
		c.pages = map[string]pornolabCachedPage{}
	}
	if c.flights == nil {
		c.flights = map[string]*pornolabFlight{}
	}
	if c.now == nil {
		c.now = time.Now
	}
}
func (c *PornolabCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initLocked()
	c.cancel()
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.epoch++
	c.pages = map[string]pornolabCachedPage{}
	c.flights = map[string]*pornolabFlight{}
}

// Context ties the entire operation, including media and final save, to privacy invalidation.
func (c *PornolabCache) Context(parent context.Context) (context.Context, context.CancelFunc) {
	c.mu.Lock()
	c.initLocked()
	generation := c.ctx
	epoch := c.epoch
	c.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(generation, cancel)
	return context.WithValue(ctx, pornolabGenerationKey{}, pornolabGeneration{c, epoch}), func() { stop(); cancel() }
}
func (c *PornolabCache) Valid(ctx context.Context) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := ctx.Value(pornolabGenerationKey{}).(pornolabGeneration)
	return ctx.Err() == nil && ok && g.cache == c && g.epoch == c.epoch
}
func (c *PornolabCache) stamp(snapshot string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strconv.FormatUint(c.epoch, 10) + ":" + snapshot
}
func (c *PornolabCache) unstamp(snapshot string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	epoch, value, ok := strings.Cut(snapshot, ":")
	if !ok || epoch != strconv.FormatUint(c.epoch, 10) {
		return "", context.Canceled
	}
	return value, nil
}

func fetchPornolab(ctx context.Context, client *http.Client, base string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", base, nil)
	if err != nil {
		return nil, requestErr("Pornolab", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, requestErr("Pornolab", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, httpStatusErr("Pornolab", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return nil, readErr(err)
	}
	if len(b) > 4<<20 {
		return nil, readErr(io.ErrShortBuffer)
	}
	return b, nil
}
func (c *PornolabCache) get(ctx context.Context, client *http.Client, base string, fresh bool) ([]byte, error) {
	c.mu.Lock()
	c.initLocked()
	now := c.now()
	for key, p := range c.pages {
		if now.Sub(p.used) >= 10*time.Minute {
			delete(c.pages, key)
		}
	}
	if p, ok := c.pages[base]; ok && !fresh {
		p.used = now
		c.pages[base] = p
		c.mu.Unlock()
		return p.data, nil
	}
	if f := c.flights[base]; f != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			return f.data, f.err
		}
	}
	f := &pornolabFlight{done: make(chan struct{})}
	c.flights[base] = f
	epoch := c.epoch
	generation := c.ctx
	c.mu.Unlock()
	requestCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(generation, cancel)
	data, err := fetchPornolab(requestCtx, client, base)
	stop()
	cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch != c.epoch || ctx.Err() != nil {
		data = nil
		err = context.Canceled
	}
	if err == nil {
		if _, ok := c.pages[base]; !ok && len(c.pages) >= 5 {
			var oldest string
			var when time.Time
			for k, p := range c.pages {
				if oldest == "" || p.used.Before(when) {
					oldest = k
					when = p.used
				}
			}
			delete(c.pages, oldest)
		}
		c.pages[base] = pornolabCachedPage{data, c.now()}
	}
	f.data, f.err = data, err
	close(f.done)
	if c.flights[base] == f {
		delete(c.flights, base)
	}
	return data, err
}
func (p *PornlabParser) document(ctx context.Context, raw string, cache *PornolabCache, fresh bool) (*goquery.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base, _, err := PornolabURL(raw)
	if err != nil {
		return nil, err
	}
	var b []byte
	if cache == nil {
		b, err = fetchPornolab(ctx, p.client, base)
	} else {
		b, err = cache.get(ctx, p.client, base, fresh)
	}
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(charmap.Windows1251.NewDecoder().Reader(bytes.NewReader(b)))
	if err != nil {
		return nil, readErr(err)
	}
	return doc, nil
}
