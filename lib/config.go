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

const (
	Success = "success"
)

// ServerConfig 서버 설정
type ServerConfig struct {
	HTTPPort string `json:"http_port"`
}

// StreamConfig 스트림 설정
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

// Segment HLS 세그먼트 캐시
type Segment struct {
	Duration time.Duration
	Data     []*av.Packet
}

// Viewer 시청자 정보
type Viewer struct {
	Channel chan av.Packet
}

// StreamManager 스트림 관리자
type StreamManager struct {
	mutex   sync.RWMutex
	Server  ServerConfig             `json:"server"`
	Streams map[string]*StreamConfig `json:"streams"`
}

// NewStreamManager 새 스트림 관리자 생성
func NewStreamManager() *StreamManager {
	return &StreamManager{
		Server:  ServerConfig{HTTPPort: "8083"},
		Streams: make(map[string]*StreamConfig),
	}
}

// AddStream 스트림 추가
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

// RemoveStream 스트림 제거
func (sm *StreamManager) RemoveStream(id string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	delete(sm.Streams, id)
}

// RunIfNotRunning 필요시 스트림 시작
func (sm *StreamManager) RunIfNotRunning(id string) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return false
	}

	if stream.OnDemand && !stream.RunLock {
		stream.RunLock = true
		return true
	}
	return false
}

// SetRunLock 스트림 잠금 상태 설정
func (sm *StreamManager) SetRunLock(id string, lock bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[id]; exists {
		stream.RunLock = lock
	}
}

// HasViewer 시청자 유무 확인
func (sm *StreamManager) HasViewer(id string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if stream, exists := sm.Streams[id]; exists && len(stream.Clients) > 0 {
		return true
	}
	return false
}

// BroadcastPacket 모든 시청자에게 패킷 전송
func (sm *StreamManager) BroadcastPacket(id string, pkt av.Packet) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return
	}

	for _, viewer := range stream.Clients {
		if len(viewer.Channel) < cap(viewer.Channel) {
			viewer.Channel <- pkt
		}
	}
}

// StreamExists 스트림 존재 확인
func (sm *StreamManager) StreamExists(id string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	_, exists := sm.Streams[id]
	return exists
}

// UpdateCodecs 코덱 정보 업데이트
func (sm *StreamManager) UpdateCodecs(id string, codecs []av.CodecData) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[id]; exists {
		stream.Codecs = codecs
	}
}

// GetCodecs 코덱 정보 가져오기
func (sm *StreamManager) GetCodecs(id string) ([]av.CodecData, error) {
	for i := 0; i < 100; i++ {
		sm.mutex.RLock()
		stream, exists := sm.Streams[id]
		sm.mutex.RUnlock()

		if !exists {
			return nil, configs.ErrStreamNotFound
		}

		if stream.Codecs != nil && len(stream.Codecs) > 0 {
			return stream.Codecs, nil
		}

		time.Sleep(50 * time.Millisecond)
	}
	return nil, configs.ErrStreamChannelCodecNotFound
}

// AddClient 클라이언트 추가
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

// RemoveClient 클라이언트 제거
func (sm *StreamManager) RemoveClient(streamID, clientID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.Streams[streamID]; exists {
		delete(stream.Clients, clientID)
	}
}

// ListStreams 스트림 목록 조회
func (sm *StreamManager) ListStreams() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make([]string, 0, len(sm.Streams))
	for id := range sm.Streams {
		result = append(result, id)
	}

	return result
}

// AddHLSSegment HLS 세그먼트 추가
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

	// 오래된 세그먼트 삭제
	if len(stream.HLSSegmentBuffer) > 6 {
		delete(stream.HLSSegmentBuffer, stream.HLSSegmentNumber-6)
	}

	return nil
}

// GetHLSM3U8 HLS M3U8 플레이리스트 가져오기
func (sm *StreamManager) GetHLSM3U8(id string) (string, int, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.Streams[id]
	if !exists {
		return "", 0, configs.ErrStreamNotFound
	}

	var playlist string
	playlist += "#EXTM3U\r\n#EXT-X-TARGETDURATION:4\r\n#EXT-X-VERSION:4\r\n"
	playlist += "#EXT-X-MEDIA-SEQUENCE:" + strconv.Itoa(stream.HLSSegmentNumber) + "\r\n"

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

// GetHLSSegment HLS 세그먼트 가져오기
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

// FlushHLSSegments HLS 세그먼트 초기화
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

// 유틸리티 함수
func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// StringToInt 문자열을 정수로 변환
func StringToInt(val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return i
}
