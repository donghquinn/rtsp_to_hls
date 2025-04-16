package lib

import (
	"log"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"org.donghyuns.com/rtsphls/configs"
)

// RTSPWorker RTSP 스트림 처리 워커
type RTSPWorker struct {
	manager       *StreamManager
	streamID      string
	url           string
	onDemand      bool
	stopChan      chan struct{}
	isRunning     bool
	reconnectTime time.Duration
}

// NewRTSPWorker 새 RTSP 워커 생성
func NewRTSPWorker(manager *StreamManager, streamID, url string, onDemand bool) *RTSPWorker {
	return &RTSPWorker{
		manager:       manager,
		streamID:      streamID,
		url:           url,
		onDemand:      onDemand,
		stopChan:      make(chan struct{}),
		isRunning:     false,
		reconnectTime: 5 * time.Second,
	}
}

// Start 워커 시작
func (w *RTSPWorker) Start() {
	if w.isRunning {
		return
	}

	w.isRunning = true
	go w.loop()
}

// Stop 워커 정지
func (w *RTSPWorker) Stop() {
	if !w.isRunning {
		return
	}

	w.isRunning = false
	close(w.stopChan)
}

// loop 메인 처리 루프
func (w *RTSPWorker) loop() {
	defer w.manager.SetRunLock(w.streamID, false)

	for w.isRunning {
		log.Printf("[%s] Stream trying to connect", w.streamID)

		err := w.processStream()
		if err != nil {
			log.Printf("[%s] Stream error: %s", w.streamID, err)
		}

		if w.onDemand && !w.manager.HasViewer(w.streamID) {
			log.Printf("[%s] Stream stopped: no viewers", w.streamID)
			break
		}

		// 재연결 전 대기
		select {
		case <-w.stopChan:
			return
		case <-time.After(w.reconnectTime):
			// 재연결 시도
		}
	}
}

// processStream RTSP 스트림 처리
func (w *RTSPWorker) processStream() error {
	keyFrameTimeout := time.NewTimer(20 * time.Second)
	clientCheckTimeout := time.NewTimer(20 * time.Second)

	var prevKeyFrameTS time.Duration
	var segmentBuffer []*av.Packet

	// RTSP 클라이언트 연결
	client, err := rtspv2.Dial(rtspv2.RTSPClientOptions{
		URL:              w.url,
		DisableAudio:     false,
		DialTimeout:      3 * time.Second,
		ReadWriteTimeout: 3 * time.Second,
		Debug:            false,
	})

	if err != nil {
		return err
	}
	defer client.Close()

	// 코덱 정보 업데이트
	if client.CodecData != nil {
		w.manager.UpdateCodecs(w.streamID, client.CodecData)
	}

	// 오디오 전용 스트림인지 확인
	var isAudioOnly bool
	if len(client.CodecData) == 1 && client.CodecData[0].Type().IsAudio() {
		isAudioOnly = true
	}

	// 메인 처리 루프
	for {
		select {
		case <-w.stopChan:
			return nil

		case <-clientCheckTimeout.C:
			if w.onDemand && !w.manager.HasViewer(w.streamID) {
				return configs.ErrStreamExitNoViewer
			}
			clientCheckTimeout.Reset(20 * time.Second)

		case <-keyFrameTimeout.C:
			return configs.ErrStreamExitNoVideoOnStream

		case signal := <-client.Signals:
			switch signal {
			case rtspv2.SignalCodecUpdate:
				w.manager.UpdateCodecs(w.streamID, client.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return configs.ErrStreamExitRtspDisconnect
			}

		case packet := <-client.OutgoingPacketQueue:
			// 키프레임이거나 오디오 전용 스트림인 경우
			if packet.IsKeyFrame || isAudioOnly {
				keyFrameTimeout.Reset(20 * time.Second)

				// 이전 세그먼트가 있으면 HLS에 추가
				if prevKeyFrameTS > 0 {
					err := w.manager.AddHLSSegment(w.streamID, segmentBuffer, packet.Time-prevKeyFrameTS)
					if err != nil {
						log.Printf("[%s] Error adding HLS segment: %s", w.streamID, err)
					}
					segmentBuffer = []*av.Packet{}
				}

				prevKeyFrameTS = packet.Time
			}

			// 세그먼트 버퍼에 패킷 추가
			segmentBuffer = append(segmentBuffer, packet)

			// 클라이언트에 패킷 브로드캐스트
			w.manager.BroadcastPacket(w.streamID, packet)
		}
	}
}
