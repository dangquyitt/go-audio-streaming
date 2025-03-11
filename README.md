# Go Audio Streaming

A simple audio streaming application using WebSocket in Go.

## Features

- Stream audio files over WebSocket
- Simple web interface with audio player
- Support for MP3 audio files
- Real-time streaming with playback controls

## Prerequisites

- Go 1.24 or higher
- Web browser with WebSocket support

## Installation

1. Clone the repository:

   ```
   git clone https://github.com/dangquyitt/go-audio-streaming.git
   cd go-audio-streaming
   ```

2. Run the application:

   ```
   go run main.go
   ```

3. Open your browser and navigate to:
   ```
   http://localhost:8080
   ```

## Usage

1. Click the "Connect" button to establish a WebSocket connection
2. Select an audio file from the dropdown menu
3. Click "Start Streaming" to begin streaming the selected audio
4. Use the audio player controls to pause/play or adjust volume
5. Click "Stop Streaming" to stop the current stream

## Project Structure

- `main.go`: Server-side implementation with WebSocket handling
- `static/index.html`: Client-side web interface
- `resource/`: Directory containing sample audio files

## How It Works

1. The server reads audio files in chunks and sends them over WebSocket
2. The client receives the audio chunks and plays them using the HTML5 audio element
3. WebSocket provides a bidirectional communication channel for control messages

## License
