package parser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

type observedDoneContext struct {
	context.Context
	seen chan struct{}
	once sync.Once
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.seen) })
	return c.Context.Done()
}

func TestPornolabCacheWaiterSurvivesOwnerCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(started)
			<-release
		}
		_, _ = w.Write([]byte("page"))
	}))
	defer srv.Close()
	defer close(release)
	cache := &PornolabCache{}
	owner, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := cache.get(owner, srv.Client(), srv.URL, false); done <- err }()
	<-started
	waiter := &observedDoneContext{Context: context.Background(), seen: make(chan struct{})}
	result := make(chan error, 1)
	go func() { _, err := cache.get(waiter, srv.Client(), srv.URL, false); result <- err }()
	<-waiter.seen
	cancel()
	if err := <-done; err == nil {
		t.Fatal("owner was not cancelled")
	}
	if err := <-result; err != nil {
		t.Fatalf("independent waiter inherited cancellation: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatal("waiter did not retry the cancelled fetch", requests.Load())
	}
}

func TestPornolabSharedYearMustBeUnambiguous(t *testing.T) {
	for _, tc := range []struct{ title, want string }{{"Release [2020, Adventure]", "2020"}, {"Release [2020, 2021]", ""}, {"Release [2020-2021]", ""}} {
		doc := collectionDoc(t, encodedCollection(t, `<h1 class="maintitle">`+tc.title+`</h1><div class="post-user-message"><span class="post-b">Описание</span>: Story</div>`))
		if got := pornolabSections(doc)[0].year; got != tc.want {
			t.Errorf("%s: year=%q, want %q", tc.title, got, tc.want)
		}
	}
}
