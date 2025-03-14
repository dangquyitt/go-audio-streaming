package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type AudioFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Duration int    `json:"duration"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

func main() {
	// CORS middleware
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			next.ServeHTTP(w, r)
		})
	}

	// Static file server for direct audio access
	fileServer := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// API to get list of audio files
	http.HandleFunc("/api/audio/list", getAudioList)

	// WebSocket endpoint for streaming audio
	http.HandleFunc("/ws/stream", streamAudio)

	// Wrap the server with CORS middleware
	wrappedHandler := corsMiddleware(http.DefaultServeMux)

	port := 8080
	fmt.Printf("Server started at http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), wrappedHandler))
}

func getAudioList(w http.ResponseWriter, r *http.Request) {
	audioDir := "static/audio"
	files, err := os.ReadDir(audioDir)
	if err != nil {
		http.Error(w, "Failed to read audio directory", http.StatusInternalServerError)
		return
	}

	var audioFiles []AudioFile
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".mp3") {
			fileInfo, err := file.Info()
			if err != nil {
				continue
			}

			// Extract duration from filename (sample-003s.mp3 -> 3 seconds)
			duration := extractDurationFromFilename(file.Name())

			audioFiles = append(audioFiles, AudioFile{
				Name:     file.Name(),
				Path:     filepath.Join("/static/audio", file.Name()),
				Size:     fileInfo.Size(),
				Duration: duration,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(audioFiles)
}

// extractDurationFromFilename parses duration from filename patterns like "sample-003s.mp3"
func extractDurationFromFilename(filename string) int {
	// Check if filename matches the pattern "sample-XXXs.mp3"
	parts := strings.Split(filename, "-")
	if len(parts) >= 2 {
		// Extract the part that should contain the duration (e.g., "003s.mp3")
		durationPart := parts[len(parts)-1]
		// Remove the "s.mp3" suffix if it exists
		durationPart = strings.TrimSuffix(durationPart, "s.mp3")

		// Parse the duration value
		var duration int
		_, err := fmt.Sscanf(durationPart, "%d", &duration)
		if err == nil {
			return duration
		}
	}
	// Default to 0 if pattern doesn't match
	return 0
}

func streamAudio(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Read the file name from the client
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Println("WebSocket read error:", err)
		return
	}

	fileName := string(message)
	filePath := filepath.Join("static/audio", fileName)

	// Check if file exists
	_, err = os.Stat(filePath)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: File not found"))
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error opening file"))
		return
	}
	defer file.Close()

	// Send the file in chunks
	buffer := make([]byte, 4096) // 4KB chunks
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte("Error reading file"))
			return
		}

		// Send the chunk
		err = conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
		if err != nil {
			log.Println("WebSocket write error:", err)
			return
		}

		// Small delay to simulate streaming
		time.Sleep(50 * time.Millisecond)
	}
}
