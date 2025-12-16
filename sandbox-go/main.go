package main

import (
	"bufio"
	"bytes" // <--- NEW
	"encoding/json"
	"fmt"
	"io" // <--- NEW
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type WasmRequest struct {
	Module string `json:"module"`
	Target string `json:"target"`
}

type SystemRequest struct {
	Cmd string `json:"cmd"`
}

/* ---------------- SSE HEADERS ---------------- */

func sendHeaders(w http.ResponseWriter) http.Flusher {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return nil
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	fmt.Fprintf(w, "data: start\n\n")
	flusher.Flush()
	return flusher
}

/* -------- ROBUST PATH FINDER -------- */

func profilePath(name string) string {
	if base := os.Getenv("SANDBOX_PROFILE_DIR"); base != "" {
		return filepath.Join(base, name)
	}
	wd, err := os.Getwd()
	if err != nil {
		return name
	}
	candidates := []string{
		filepath.Join(wd, "sandbox_profiles", name),
		filepath.Join(wd, "sandbox-go", "sandbox_profiles", name),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	log.Printf("❌ CRITICAL: Could not find profile '%s'", name)
	return name
}

/* ---------------- SYSTEM HANDLER ---------------- */

/* ---------------- SYSTEM HANDLER ---------------- */

func systemHandler(w http.ResponseWriter, r *http.Request) {
	// ★★★ STEP 1: Read the request body BEFORE sending any headers ★★★

	// Debugging: Print what we received
	bodyBytes, _ := io.ReadAll(r.Body)
	// r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Not needed if we decode buffer directly below
	log.Printf("📦 RECEIVED BODY: '%s'", string(bodyBytes))

	var req SystemRequest
	// Decode from the bytes we just read
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&req); err != nil {
		log.Printf("❌ JSON DECODE ERROR: %v", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Cmd == "" {
		log.Printf("⚠️ WARNING: Received empty command!")
		http.Error(w, "Command cannot be empty", http.StatusBadRequest)
		return
	}

	// ★★★ STEP 2: NOW we send the headers and start the stream ★★★
	flusher := sendHeaders(w)
	if flusher == nil {
		return
	}

	// Use the robust path finder
	profile := profilePath("system.profile")
	log.Printf("🚀 Executing: %s (Profile: %s)", req.Cmd, profile)

	cmd := exec.Command(
		"firejail",
		"--quiet",
		"--profile="+profile,
		"stdbuf", "-oL", "-eL",
		"bash", "-c", req.Cmd,
	)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(w, "data: Error starting command: %s\n\n", err)
		flusher.Flush()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	scanOut := bufio.NewScanner(stdout)
	scanErr := bufio.NewScanner(stderr)

	go func() {
		defer wg.Done()
		for scanOut.Scan() {
			fmt.Fprintf(w, "data: %s\n\n", scanOut.Text())
			flusher.Flush()
		}
	}()

	go func() {
		defer wg.Done()
		for scanErr.Scan() {
			fmt.Fprintf(w, "data: ERR: %s\n\n", scanErr.Text())
			flusher.Flush()
		}
	}()

	cmd.Wait()
	wg.Wait()

	fmt.Fprintf(w, "data: DONE\n\n")
	flusher.Flush()
}

/* ---------------- MAIN ---------------- */

func main() {
	http.HandleFunc("/run-wasm", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/run-system", systemHandler)

	log.Println("🔥 Go sandbox listening on :9000 (Firejail enabled)")
	log.Fatal(http.ListenAndServe(":9000", nil))
}
