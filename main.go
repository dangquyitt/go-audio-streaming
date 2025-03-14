package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing
	},
}

// Prometheus metrics
var (
	activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "audio_streaming_active_connections",
		Help: "The current number of active WebSocket connections",
	})

	streamingCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audio_streaming_total_streams",
		Help: "The total number of audio streams initiated",
	})

	audioChunksSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audio_streaming_chunks_sent",
		Help: "The total number of audio chunks sent",
	})

	audioStreamDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "audio_streaming_duration_seconds",
		Help:    "The duration of audio streams in seconds",
		Buckets: prometheus.LinearBuckets(1, 5, 10), // 1-50 seconds
	})

	websocketErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audio_streaming_websocket_errors",
		Help: "The total number of WebSocket errors",
	})
)

// Client represents a connected WebSocket client
type Client struct {
	conn       *websocket.Conn
	mu         sync.Mutex
	streaming  bool
	stopCh     chan struct{}
	startTime  time.Time
	totalBytes int
	clientIP   string
	audioFile  string
}

// Message represents a WebSocket message
type Message struct {
	Action   string `json:"action"`
	Filename string `json:"filename,omitempty"`
}

// StatusMessage represents a status message to send to the client
type StatusMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func main() {
	mux := http.NewServeMux()

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	// Expose Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// List available audio files
	mux.HandleFunc("/audios", func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir("./resource")
		if err != nil {
			http.Error(w, "Failed to read audio directory", http.StatusInternalServerError)
			return
		}

		var audioFiles []string
		for _, file := range files {
			if !file.IsDir() && (filepath.Ext(file.Name()) == ".mp3" || filepath.Ext(file.Name()) == ".wav") {
				audioFiles = append(audioFiles, file.Name())
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(audioFiles)
	})

	// WebSocket endpoint
	mux.HandleFunc("/ws", handleWebSocket)

	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Starting server on port 8080")
	log.Println("Metrics available at http://localhost:8080/metrics")
	log.Println("Open http://localhost:8080 in your browser")
	log.Fatal(server.ListenAndServe())
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		websocketErrors.Inc()
		return
	}
	defer conn.Close()

	// Increment active connections counter
	activeConnections.Inc()
	defer activeConnections.Dec()

	clientIP := r.RemoteAddr
	log.Printf("New WebSocket connection established from %s", clientIP)

	client := &Client{
		conn:      conn,
		streaming: false,
		stopCh:    make(chan struct{}),
		clientIP:  clientIP,
	}

	// Listen for messages from the client
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			websocketErrors.Inc()
			break
		}

		if messageType == websocket.TextMessage {
			var msg Message
			if err := json.Unmarshal(p, &msg); err != nil {
				log.Println("Error unmarshaling message:", err)
				websocketErrors.Inc()
				continue
			}

			switch msg.Action {
			case "start":
				if msg.Filename == "" {
					sendStatusMessage(client, "Error: No filename provided")
					websocketErrors.Inc()
					continue
				}

				// Stop any existing streaming
				if client.streaming {
					client.mu.Lock()
					close(client.stopCh)
					client.stopCh = make(chan struct{})
					client.mu.Unlock()

					// Record duration of previous stream
					duration := time.Since(client.startTime).Seconds()
					audioStreamDuration.Observe(duration)
				}

				client.streaming = true
				client.startTime = time.Now()
				client.totalBytes = 0
				client.audioFile = msg.Filename

				// Increment streaming counter
				streamingCounter.Inc()

				go streamAudio(client, msg.Filename)

			case "stop":
				if client.streaming {
					client.mu.Lock()
					close(client.stopCh)
					client.stopCh = make(chan struct{})
					client.streaming = false
					client.mu.Unlock()

					// Record duration
					duration := time.Since(client.startTime).Seconds()
					audioStreamDuration.Observe(duration)

					sendStatusMessage(client, "Streaming stopped")
				}
			}
		}
	}
}

func streamAudio(client *Client, filename string) {
	filePath := filepath.Join("resource", filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		sendStatusMessage(client, fmt.Sprintf("Error: File %s not found", filename))
		websocketErrors.Inc()
		return
	}

	sendStatusMessage(client, fmt.Sprintf("Streaming %s", filename))

	file, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file:", err)
		sendStatusMessage(client, "Error opening audio file")
		websocketErrors.Inc()
		return
	}
	defer file.Close()

	// Get file info for logging
	fileInfo, err := file.Stat()
	if err == nil {
		log.Printf("Streaming file: %s (size: %d bytes)", filename, fileInfo.Size())
	}

	// Buffer for reading chunks of the audio file
	buffer := make([]byte, 8192) // 8KB chunks - tối ưu cho streaming audio

	for {
		select {
		case <-client.stopCh:
			log.Printf("Streaming stopped for file: %s", filename)
			return
		default:
			n, err := file.Read(buffer)
			if err == io.EOF {
				log.Printf("Finished streaming file: %s", filename)
				sendStatusMessage(client, "Streaming finished")

				// Record duration when finished
				duration := time.Since(client.startTime).Seconds()
				audioStreamDuration.Observe(duration)

				client.streaming = false
				return
			}
			if err != nil {
				log.Println("Error reading file:", err)
				sendStatusMessage(client, "Error reading audio file")
				websocketErrors.Inc()
				client.streaming = false
				return
			}

			client.mu.Lock()
			err = client.conn.WriteMessage(websocket.BinaryMessage, buffer[:n])
			client.mu.Unlock()

			if err != nil {
				log.Println("Error writing to WebSocket:", err)
				websocketErrors.Inc()
				client.streaming = false
				return
			}

			// Increment metrics
			audioChunksSent.Inc()
			client.totalBytes += n

			// Giảm delay để stream mượt hơn
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func sendStatusMessage(client *Client, status string) {
	msg := StatusMessage{
		Type: "status",
		Data: status,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Println("Error marshaling status message:", err)
		websocketErrors.Inc()
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	if err := client.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Println("Error sending status message:", err)
		websocketErrors.Inc()
	}
}
