"use client";

import { useState, useEffect, useRef } from "react";

type AudioFile = {
  name: string;
  path: string;
  size: number;
  duration: number;
};

type PlaybackMode = "streaming" | "direct";

// Utility function to format seconds as MM:SS
const formatDuration = (seconds: number): string => {
  if (!seconds) return "00:00";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins.toString().padStart(2, "0")}:${secs
    .toString()
    .padStart(2, "0")}`;
};

export default function Home() {
  const [audioFiles, setAudioFiles] = useState<AudioFile[]>([]);
  const [selectedFile, setSelectedFile] = useState<AudioFile | null>(null);
  const [playbackMode, setPlaybackMode] = useState<PlaybackMode>("streaming");
  const [isPlaying, setIsPlaying] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const audioRef = useRef<HTMLAudioElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const mediaSourceRef = useRef<MediaSource | null>(null);
  const sourceBufferRef = useRef<SourceBuffer | null>(null);
  const chunksRef = useRef<Uint8Array[]>([]);

  // Fetch audio files from API
  useEffect(() => {
    const fetchAudioFiles = async () => {
      try {
        setLoading(true);
        const response = await fetch("http://localhost:8080/api/audio/list");
        if (!response.ok) {
          throw new Error("Failed to fetch audio files");
        }
        const data = await response.json();
        setAudioFiles(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error occurred");
        console.error("Error fetching audio files:", err);
      } finally {
        setLoading(false);
      }
    };

    fetchAudioFiles();
  }, []);

  // Handle direct playback
  const playDirectAudio = () => {
    if (!selectedFile || !audioRef.current) return;

    // Set audio source to direct file URL
    audioRef.current.src = `http://localhost:8080${selectedFile.path}`;
    audioRef.current.load();
    audioRef.current.play();
    setIsPlaying(true);
  };

  // Handle streaming via WebSocket
  const playStreamingAudio = () => {
    if (!selectedFile) return;

    // Clean up any existing connections
    if (wsRef.current) {
      wsRef.current.close();
    }

    try {
      // Initialize MediaSource
      mediaSourceRef.current = new MediaSource();
      if (audioRef.current) {
        audioRef.current.src = URL.createObjectURL(mediaSourceRef.current);
      }

      chunksRef.current = [];

      mediaSourceRef.current.addEventListener("sourceopen", () => {
        try {
          // Create source buffer
          sourceBufferRef.current =
            mediaSourceRef.current!.addSourceBuffer("audio/mpeg");

          // Connect to WebSocket
          wsRef.current = new WebSocket("ws://localhost:8080/ws/stream");

          wsRef.current.onopen = () => {
            // Send the filename to request streaming
            wsRef.current!.send(selectedFile.name);
          };

          wsRef.current.onmessage = (event) => {
            // Process incoming binary data
            if (event.data instanceof Blob) {
              event.data.arrayBuffer().then((buffer) => {
                const chunk = new Uint8Array(buffer);
                chunksRef.current.push(chunk);

                // Append to source buffer if ready
                if (
                  sourceBufferRef.current &&
                  !sourceBufferRef.current.updating
                ) {
                  try {
                    sourceBufferRef.current.appendBuffer(chunk);
                  } catch (e) {
                    console.error("Error appending buffer:", e);
                  }
                }
              });
            }
          };

          wsRef.current.onerror = (error) => {
            console.error("WebSocket error:", error);
            setError("WebSocket connection error");
          };

          wsRef.current.onclose = () => {
            // End of stream
            if (
              mediaSourceRef.current &&
              mediaSourceRef.current.readyState === "open"
            ) {
              try {
                mediaSourceRef.current.endOfStream();
              } catch (e) {
                console.error("Error ending stream:", e);
              }
            }
          };
        } catch (e) {
          console.error("Error setting up media source:", e);
          setError("Error setting up media streaming");
        }
      });

      // Start playing
      if (audioRef.current) {
        audioRef.current
          .play()
          .then(() => {
            setIsPlaying(true);
          })
          .catch((err) => {
            console.error("Error playing audio:", err);
          });
      }
    } catch (e) {
      console.error("Streaming error:", e);
      setError("Error initializing streaming playback");
    }
  };

  // Play audio based on selected mode
  const playAudio = () => {
    if (playbackMode === "direct") {
      playDirectAudio();
    } else {
      playStreamingAudio();
    }
  };

  // Handle file selection
  const handleFileSelect = (file: AudioFile) => {
    // Stop any current playback
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.currentTime = 0;
    }

    // Close WebSocket if open
    if (wsRef.current) {
      wsRef.current.close();
    }

    setSelectedFile(file);
    setIsPlaying(false);
  };

  // Handle playback mode change
  const handleModeChange = (mode: PlaybackMode) => {
    // Stop any current playback
    if (audioRef.current) {
      audioRef.current.pause();
      audioRef.current.currentTime = 0;
    }

    // Close WebSocket if open
    if (wsRef.current) {
      wsRef.current.close();
    }

    setPlaybackMode(mode);
    setIsPlaying(false);
  };

  return (
    <main className="min-h-screen p-8 bg-gray-100">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-3xl font-bold mb-8 text-center">Audio Player</h1>

        {error && (
          <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
            <p>{error}</p>
          </div>
        )}

        <div className="flex flex-col md:flex-row gap-6">
          {/* Audio Files List */}
          <div className="w-full md:w-1/2 bg-white p-4 rounded shadow">
            <h2 className="text-xl font-semibold mb-4">Audio Files</h2>

            {loading ? (
              <p className="text-gray-500">Loading audio files...</p>
            ) : audioFiles.length === 0 ? (
              <p className="text-gray-500">No audio files found</p>
            ) : (
              <ul className="divide-y">
                {audioFiles.map((file) => (
                  <li
                    key={file.name}
                    className={`py-2 px-3 cursor-pointer hover:bg-gray-100 ${
                      selectedFile?.name === file.name
                        ? "bg-blue-50 border-l-4 border-blue-500"
                        : ""
                    }`}
                    onClick={() => handleFileSelect(file)}
                  >
                    <p className="font-medium">{file.name}</p>
                    <p className="text-sm text-gray-500">
                      Size: {Math.round(file.size / 1024)} KB
                    </p>
                    <p className="text-sm text-gray-500">
                      Duration: {formatDuration(file.duration)}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {/* Player Controls */}
          <div className="w-full md:w-1/2 bg-white p-4 rounded shadow">
            <h2 className="text-xl font-semibold mb-4">Player Controls</h2>

            {/* Playback mode selection */}
            <div className="mb-4">
              <h3 className="font-medium mb-2">Playback Mode:</h3>
              <div className="flex gap-4">
                <button
                  className={`px-4 py-2 rounded ${
                    playbackMode === "streaming"
                      ? "bg-blue-500 text-white"
                      : "bg-gray-200"
                  }`}
                  onClick={() => handleModeChange("streaming")}
                >
                  Streaming
                </button>
                <button
                  className={`px-4 py-2 rounded ${
                    playbackMode === "direct"
                      ? "bg-blue-500 text-white"
                      : "bg-gray-200"
                  }`}
                  onClick={() => handleModeChange("direct")}
                >
                  Direct Playback
                </button>
              </div>
            </div>

            {/* Currently playing */}
            <div className="mb-4">
              <h3 className="font-medium mb-2">Currently Selected:</h3>
              {selectedFile ? (
                <div>
                  <p className="text-gray-700">{selectedFile.name}</p>
                  <p className="text-sm text-gray-500">
                    Duration: {formatDuration(selectedFile.duration)}
                  </p>
                </div>
              ) : (
                <p className="text-gray-500">No file selected</p>
              )}
            </div>

            {/* Audio element (hidden) */}
            <audio
              ref={audioRef}
              controls
              className="w-full mt-2"
              onEnded={() => setIsPlaying(false)}
            />

            {/* Play button */}
            <button
              className={`w-full mt-4 py-2 rounded font-medium ${
                selectedFile
                  ? "bg-green-500 hover:bg-green-600 text-white"
                  : "bg-gray-300 text-gray-500 cursor-not-allowed"
              }`}
              disabled={!selectedFile}
              onClick={playAudio}
            >
              {isPlaying ? "Playing..." : "Play"}
            </button>

            {/* Mode explanation */}
            <div className="mt-6 text-sm text-gray-600 bg-gray-50 p-3 rounded">
              <h4 className="font-medium mb-1">About Playback Modes:</h4>
              <p className="mb-2">
                <strong>Direct Playback:</strong> Loads the entire audio file
                before playing. Faster initial load for small files.
              </p>
              <p>
                <strong>Streaming:</strong> Plays audio as it's received in
                chunks. Better for large files and network efficiency.
              </p>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
