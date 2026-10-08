package realtime

import (
	"testing"

	"github.com/google/uuid"
)

func TestHubDeliversOnlyToTargetUsers(t *testing.T) {
	h := NewHub()
	a, b := uuid.New(), uuid.New()
	ca, cb := h.Register(a), h.Register(b)

	h.NotifyRevision([]uuid.UUID{a}, 7)
	select {
	case msg := <-ca.Send:
		if string(msg) != `{"revision":7,"type":"revision"}` {
			t.Fatalf("got %s", msg)
		}
	default:
		t.Fatal("user a got nothing")
	}
	select {
	case <-cb.Send:
		t.Fatal("user b must not be notified")
	default:
	}

	h.Unregister(ca)
	if h.Count(a) != 0 {
		t.Fatal("unregister failed")
	}
}

func TestHubDoesNotBlockOnSlowClient(t *testing.T) {
	h := NewHub()
	u := uuid.New()
	h.Register(u) // никто не читает
	for i := 0; i < 100; i++ {
		h.NotifyRevision([]uuid.UUID{u}, int64(i))
	}
}
