package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

type server struct {
	mu         sync.Mutex
	account    string
	apiKey     string
	ip         string
	recordName string
	recordID   int
}

type record struct {
	ID   int    `json:"record_id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
	TTL  int    `json:"ttl"`
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL.RequestURI())
	if r.URL.Path == "/myip" {
		s.mu.Lock()
		defer s.mu.Unlock()
		fmt.Fprintln(w, s.ip)
		return
	}
	if r.URL.Path == "/nic/update" {
		s.handleDynDNS(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/2/my/products/") {
		s.handleREST(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *server) authorized(r *http.Request) bool {
	user, password, ok := r.BasicAuth()
	return ok && user == s.account && password == s.apiKey
}

func (s *server) handleDynDNS(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, "badauth")
		return
	}
	hostname := r.URL.Query().Get("hostname")
	ip := r.URL.Query().Get("myip")
	if hostname == "" || ip == "" {
		fmt.Fprintln(w, "nohost")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recordName != hostname {
		fmt.Fprintln(w, "nohost")
		return
	}
	if s.ip == ip {
		fmt.Fprintf(w, "nochg %s\n", ip)
		return
	}
	s.ip = ip
	fmt.Fprintf(w, "good %s\n", ip)
}

func (s *server) handleREST(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+s.apiKey {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/dns/records/") {
		_ = json.NewEncoder(w).Encode(map[string]any{"records": []record{{ID: s.recordID, Name: s.recordName, Type: "A", Data: s.ip, TTL: 3600}}})
		return
	}
	if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/dns/records/") {
		var payload struct {
			Name string `json:"name"`
			Data string `json:"data"`
			Type string `json:"type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Type != "A" || payload.Name != s.recordName {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid record"})
			return
		}
		s.ip = payload.Data
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}
	http.NotFound(w, r)
}

func main() {
	listen := flag.String("listen", "127.0.0.1:18080", "listen address")
	account := flag.String("account", "test", "test account")
	apiKey := flag.String("api-key", "test_api_key", "test API key")
	ip := flag.String("ip", "198.51.100.10", "initial simulated public IPv4")
	recordName := flag.String("record", "home.example.com", "record hostname")
	recordID := flag.Int("record-id", 1, "simulated record ID")
	flag.Parse()

	s := &server{account: *account, apiKey: *apiKey, ip: *ip, recordName: *recordName, recordID: *recordID}
	httpServer := &http.Server{Addr: *listen, Handler: s}
	go func() {
		log.Printf("Simply mock listening on http://%s", *listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	_ = httpServer.Close()
	log.Printf("Simply mock stopped (record ID %d)", *recordID)
}
