package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type demoServer struct {
	product string
}

func main() {
	server := &demoServer{product: getenv("DEMO_PRODUCT", "demo-a")}
	handler := http.NewServeMux()
	handler.HandleFunc("/healthz", server.health)
	handler.HandleFunc("/api/dashboard", server.dashboard)

	log.Printf("%s product listening on :8080", server.product)

	httpServer := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Fatal(httpServer.ListenAndServe())
}

func (s *demoServer) health(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok", "product": s.product})
}

func (s *demoServer) dashboard(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeJSON(writer, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	writeJSON(writer, http.StatusOK, map[string]string{
		"product": s.product,
		"message": "dashboard data",
		"subject": request.Header.Get("x-user-id"),
		"tenant":  request.Header.Get("x-tenant-id"),
	})
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
