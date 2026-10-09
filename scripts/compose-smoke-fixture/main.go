package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
)

type fixture struct {
	mu           sync.RWMutex
	atomTitle    string
	rssTitle     string
	blockAtom    bool
	atomActive   int
	atomCanceled int
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		response, err := http.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			os.Exit(1)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			os.Exit(1)
		}
		return
	}
	server := &fixture{atomTitle: "ATOM_V1", rssTitle: "RSS_V1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/set/atom", func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.atomTitle = r.URL.Query().Get("title")
		server.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/set/rss", func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.rssTitle = r.URL.Query().Get("title")
		server.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/set/block-atom", func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		server.blockAtom = r.URL.Query().Get("enabled") == "true"
		server.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/state", func(w http.ResponseWriter, _ *http.Request) {
		server.mu.RLock()
		state := struct {
			AtomActive   int `json:"atom_active"`
			AtomCanceled int `json:"atom_canceled"`
		}{AtomActive: server.atomActive, AtomCanceled: server.atomCanceled}
		server.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	})
	mux.HandleFunc("/atom.xml", func(w http.ResponseWriter, request *http.Request) {
		server.mu.RLock()
		title := server.atomTitle
		blocked := server.blockAtom
		server.mu.RUnlock()
		if blocked {
			server.mu.Lock()
			server.atomActive++
			server.mu.Unlock()
			<-request.Context().Done()
			server.mu.Lock()
			server.atomActive--
			server.atomCanceled++
			server.mu.Unlock()
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<feed xmlns="http://www.w3.org/2005/Atom"><title>Smoke Atom</title><entry><id>stable-atom-id</id><title>%s</title><link href="https://example.test/atom-entry"/><content>Atom fixture content</content></entry></feed>`, title)
	})
	mux.HandleFunc("/rss.xml", func(w http.ResponseWriter, _ *http.Request) {
		server.mu.RLock()
		title := server.rssTitle
		server.mu.RUnlock()
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<rss version="2.0"><channel><title>Smoke RSS</title><link>https://example.test/</link><item><guid>stable-rss-guid</guid><title>%s</title><link>https://example.test/rss-entry</link><description>RSS fixture content</description></item></channel></rss>`, title)
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Fprintln(os.Stderr, "fixture server:", err)
		os.Exit(1)
	}
}
