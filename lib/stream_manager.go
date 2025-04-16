package lib

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"org.donghyuns.com/rtsphls/configs"
)

// ServerConfig represents server configuration
type ServerConfig struct {
	HTTPPort string `json:"http_port"`
}

// StreamConfig represents configuration for a single stream
type StreamConfig struct {
	URL              string            `json:"url"`
	Status           bool              `json:"status"`
	OnDemand         bool              `json:"on_demand"`
	RunLock          bool              `json:"-"`
	HLSSegmentNumber int               `json:"-"`
	HLSSegmentBuffer map[int]*Segment  `json:"-"`
	Codecs           []av.CodecData    `json:"-"`
	Clients          map[string]Viewer `json:"-"`
}

// Segment represents a cached HLS segment
type Segment struct {
	Duration time.Duration
	Data     []*av.Packet
}

// Viewer represents a connected client
type Viewer struct {
	Channel chan av.Packet
}

// StreamManager manages multiple streams
type StreamManager struct {
	mutex   sync.RWMutex
	Server  ServerConfig             `json:"server"`
	Streams map[string]*StreamConfig `json:"streams"`
}

// NewStreamManager creates a new stream manager instance
func NewStreamManager() *StreamManager {
	return &StreamManager{
		Server: ServerConfig{
			// Use configurable port or default
			HTTPPort: configs.GlobalConfig.AppPort,
		},
		Streams: make(map[string]*StreamConfig),
	}
}

// AddStream adds a new stream to the manager
func (sm *StreamManager) AddStream(id string, url string, onDemand bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.Streams[id] = &StreamConfig{
		URL:              url,
		Status:           false,
		OnDemand:         onDemand,
		RunLock:          false,
		HLSSegmentNumber: 0,
		HLSSegmentBuffer: make(map[int]*Segment),
		Codecs:           []av.CodecData{},
		Clients:          make(map[string]Viewer),
	}
}

// GetStream returns a stream by ID
func (sm *StreamManager) GetStream(id string) (*StreamConfig, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return nil, configs.ErrStreamNotFound
	}

	return stream, nil
}

// RemoveStream removes a stream from the manager
func (sm *StreamManager) RemoveStream(id string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Close all client channels
	if stream, exists := sm.Streams[id]; exists {
		for _, viewer := range stream.Clients {
			close(viewer.Channel)
		}
	}

	delete(sm.Streams, id)
}

// RunIfNotRunning starts a stream if it's not already running
func (sm *StreamManager) RunIfNotRunning(id string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return false
	}

	if !stream.RunLock {
		stream.RunLock = true
		return true
	}
	return false
}

// SetRunLock sets the run lock state for a stream
func (sm *StreamManager) SetRunLock(id string, lock bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[id]; exists {
		stream.RunLock = lock
	}
}

// HasViewer checks if a stream has any viewers
func (sm *StreamManager) HasViewer(id string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if stream, exists := sm.Streams[id]; exists && len(stream.Clients) > 0 {
		return true
	}
	return false
}

// BroadcastPacket sends a packet to all viewers of a stream
func (sm *StreamManager) BroadcastPacket(id string, pkt av.Packet) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return
	}

	for _, viewer := range stream.Clients {
		if len(viewer.Channel) < cap(viewer.Channel) {
			select {
			case viewer.Channel <- pkt:
				// Packet sent successfully
			default:
				// Channel buffer full, skip this packet
			}
		}
	}
}

// StreamExists checks if a stream exists
func (sm *StreamManager) StreamExists(id string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	_, exists := sm.Streams[id]
	return exists
}

// UpdateCodecs updates codec information for a stream
func (sm *StreamManager) UpdateCodecs(id string, codecs []av.CodecData) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[id]; exists {
		stream.Codecs = codecs
	}
}

// GetCodecs retrieves codec information for a stream
func (sm *StreamManager) GetCodecs(id string) ([]av.CodecData, error) {
	// Try multiple times with short delay in case stream is initializing
	const maxRetries = 100
	const retryInterval = 50 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		sm.mutex.RLock()
		stream, exists := sm.Streams[id]

		if !exists {
			sm.mutex.RUnlock()
			return nil, configs.ErrStreamNotFound
		}

		if stream.Codecs != nil && len(stream.Codecs) > 0 {
			codecs := stream.Codecs
			sm.mutex.RUnlock()
			return codecs, nil
		}

		sm.mutex.RUnlock()
		time.Sleep(retryInterval)
	}

	return nil, configs.ErrStreamChannelCodecNotFound
}

// AddClient adds a new client to a stream
func (sm *StreamManager) AddClient(id string) (string, chan av.Packet, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return "", nil, configs.ErrStreamNotFound
	}

	clientID := generateUUID()
	ch := make(chan av.Packet, 100)
	stream.Clients[clientID] = Viewer{Channel: ch}

	return clientID, ch, nil
}

// RemoveClient removes a client from a stream
func (sm *StreamManager) RemoveClient(streamID, clientID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[streamID]; exists {
		if viewer, found := stream.Clients[clientID]; found {
			close(viewer.Channel)
			delete(stream.Clients, clientID)
		}
	}
}

// ListStreams returns a list of all stream IDs
func (sm *StreamManager) ListStreams() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make([]string, 0, len(sm.Streams))
	for id := range sm.Streams {
		result = append(result, id)
	}

	sort.Strings(result)
	return result
}

// AddHLSSegment adds a new HLS segment to a stream
func (sm *StreamManager) AddHLSSegment(id string, packets []*av.Packet, duration time.Duration) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return configs.ErrStreamNotFound
	}

	stream.HLSSegmentNumber++
	stream.HLSSegmentBuffer[stream.HLSSegmentNumber] = &Segment{
		Duration: duration,
		Data:     packets,
	}

	// Cleanup old segments (keep last 6)
	const maxSegments = 6
	if len(stream.HLSSegmentBuffer) > maxSegments {
		// Find oldest segment(s) to remove
		var keys []int
		for k := range stream.HLSSegmentBuffer {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		// Remove all but the latest maxSegments
		for i := 0; i < len(keys)-maxSegments; i++ {
			delete(stream.HLSSegmentBuffer, keys[i])
		}
	}

	return nil
}

// GetHLSM3U8 generates an M3U8 playlist for a stream
func (sm *StreamManager) GetHLSM3U8(id string) (string, int, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return "", 0, configs.ErrStreamNotFound
	}

	var playlist string
	playlist += "#EXTM3U\r\n"
	playlist += "#EXT-X-TARGETDURATION:4\r\n"
	playlist += "#EXT-X-VERSION:4\r\n"
	playlist += "#EXT-X-MEDIA-SEQUENCE:" + strconv.Itoa(stream.HLSSegmentNumber-len(stream.HLSSegmentBuffer)+1) + "\r\n"

	var keys []int
	for k := range stream.HLSSegmentBuffer {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	segmentCount := 0
	for _, i := range keys {
		segmentCount++
		duration := strconv.FormatFloat(stream.HLSSegmentBuffer[i].Duration.Seconds(), 'f', 1, 64)
		playlist += "#EXTINF:" + duration + ",\r\n"
		playlist += "segment/" + strconv.Itoa(i) + "/file.ts\r\n"
	}

	return playlist, segmentCount, nil
}

// GetHLSSegment retrieves a specific HLS segment
func (sm *StreamManager) GetHLSSegment(id string, seq int) ([]*av.Packet, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return nil, configs.ErrStreamNotFound
	}

	segment, exists := stream.HLSSegmentBuffer[seq]
	if !exists {
		return nil, configs.ErrStreamNotHLSSegments
	}

	return segment.Data, nil
}

// FlushHLSSegments removes all HLS segments for a stream
func (sm *StreamManager) FlushHLSSegments(id string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return configs.ErrStreamNotFound
	}

	stream.HLSSegmentBuffer = make(map[int]*Segment)
	stream.HLSSegmentNumber = 0

	return nil
}

// Utility functions

// generateUUID generates a unique identifier
func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// getEnvOrDefault gets an environment variable or returns the default
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := configs.GetEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
