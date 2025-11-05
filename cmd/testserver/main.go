//nolint:errcheck,forbidigo,gosec // test utility allows simpler error handling and direct output
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: testserver [options] <current-schedule.html> [next-schedule.html]")
		fmt.Println("\nOptions:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	currentPath := args[0]
	var nextPath string
	if len(args) > 1 {
		nextPath = args[1]
	}

	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		log.Fatalf("Current schedule file does not exist: %s", currentPath)
	}

	if nextPath != "" {
		if _, err := os.Stat(nextPath); os.IsNotExist(err) {
			log.Fatalf("Next schedule file does not exist: %s", nextPath)
		}
	}

	http.HandleFunc("/shutdowns/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("next") {
			if nextPath == "" {
				http.Error(w, "Next schedule not configured", http.StatusNotFound)
				log.Printf("Next schedule requested but not configured")
				return
			}
			serveHTMLFile(w, nextPath)
		} else {
			serveHTMLFile(w, currentPath)
		}
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Test server listening on %s", addr)
	log.Printf("Current schedule: %s -> http://localhost%s/shutdowns/", currentPath, addr)
	if nextPath != "" {
		log.Printf("Next schedule: %s -> http://localhost%s/shutdowns/?next", nextPath, addr)
	}
	log.Println("\nFiles are read on each request, so you can edit them while the server is running.")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func serveHTMLFile(w http.ResponseWriter, path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		log.Printf("Error reading %s: %v", path, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
	log.Printf("Served %s (%d bytes)", path, len(content))
}
