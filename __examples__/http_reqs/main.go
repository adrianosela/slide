package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/adrianosela/slide"
)

const (
	janitorInterval = time.Second * 2
	idleTimeout     = time.Second * 5

	listenAddr = ":8008"
)

type session struct {
	id       string
	sourceIP string
	start    time.Time
	reqs     *atomic.Int32
}

func main() {
	sessions := make(map[string]*session)

	tracker := slide.NewTracker[*session](
		getSessionInitFunc(sessions),
		slide.WithJanitorInterval[*session](janitorInterval),
		slide.WithInactivityTimeout[*session](idleTimeout),
		slide.WithOnSessionEnd(getOnSessionEnd(sessions)),
	)
	defer tracker.Stop()

	log.Printf(
		"Listening on %s. Janitor interval is %s. Idle timeout is %s.",
		listenAddr,
		janitorInterval.String(),
		idleTimeout.String(),
	)
	http.ListenAndServe(listenAddr, getHandler(tracker))
}

func getSessionInitFunc(sessions map[string]*session) slide.SessionInitFunc[*session] {
	return func(sourceIP string) *session {
		return &session{
			id:       freshID(),
			sourceIP: sourceIP,
			start:    time.Now(),
			reqs:     &atomic.Int32{},
		}
	}
}

func getOnSessionEnd(sessions map[string]*session) slide.OnEndFunc[*session] {
	return func(sess *session, metadata *slide.SessionMetadata) {
		delete(sessions, sess.id)
		log.Printf(
			"finished session %s with %d requests, lasted %dms",
			sess.id,
			sess.reqs.Load(),
			int(metadata.Updated.Sub(metadata.Created).Milliseconds()),
		)
	}
}

func getHandler(tracker slide.Tracker[*session]) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// sessions will be deduplicated by source IP
		ap, _ := netip.ParseAddrPort(r.RemoteAddr)
		dedupKey := ap.Addr().String()

		// new uuid for this specific http request
		requestID := freshID()

		sess := tracker.EventStart(dedupKey, requestID).Data()
		defer tracker.EventEnd(requestID)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"session_id": "%s", "src_ip": "%s", "reqs": %d}`, sess.id, sess.sourceIP, sess.reqs.Add(1))))
	})
}

func freshID() string {
	buf := make([]byte, 10)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
