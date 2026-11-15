package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTerrainReadsDoNotBlockEditsAndReauthorizeResults(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "parallel-edit", true: "revoked-session"}[revoke], func(t *testing.T) {
			s, db, owner, _, d := secureFixture(t)
			s.sessions["terrain-read"] = &session{creatorID: owner.u.ID, last: time.Now()}
			started, finish := make(chan struct{}), make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(finish) }) }
			defer release()
			s.mux = http.NewServeMux()
			s.mux.HandleFunc("GET /api/sessions/{id}/terrain/sample", func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-finish
				respond(w, 200, map[string]string{"sample": "private terrain"})
			})
			s.mux.HandleFunc("POST /api/worlds/{world}", s.commandHistory)
			result := make(chan *httptest.ResponseRecorder, 1)
			go func() { result <- secured(s, owner, "GET", "/api/sessions/terrain-read/terrain/sample", "") }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("terrain read did not start")
			}
			if revoke {
				if err := db.RevokeSession(context.Background(), owner.secret); err != nil {
					t.Fatal(err)
				}
			} else {
				edited := make(chan *httptest.ResponseRecorder, 1)
				go func() {
					edited <- secured(s, owner, "POST", "/api/worlds/"+d.World.ID, `{"kind":"rename-world","name":"Edited during sampling","revision":1}`)
				}()
				select {
				case response := <-edited:
					if response.Code != 200 {
						t.Fatal(response.Code, response.Body.String())
					}
				case <-time.After(3 * time.Second):
					t.Fatal("terrain sampling held the shared edit gate")
				}
			}
			release()
			select {
			case response := <-result:
				if revoke {
					if response.Code != 401 || strings.Contains(response.Body.String(), "private terrain") {
						t.Fatal("revoked session received sampled data", response.Code, response.Body.String())
					}
				} else if response.Code != 200 {
					t.Fatal(response.Code, response.Body.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("terrain read did not finish")
			}
		})
	}
}
