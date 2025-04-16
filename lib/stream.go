package lib

import (
	"log"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"org.donghyuns.com/rtsphls/configs"
)

// RTSPWorker handles RTSP stream processing
type RTSPWorker struct {
	manager       *StreamManager
	streamID      string
	url           string
	onDemand      bool
	stopChan      chan struct{}
	isRunning     bool
	reconnectTime time.Duration
}

// NewRTSPWorker creates a new RTSP worker
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

// Start begins the worker's processing loop
func (w *RTSPWorker) Start() {
	if w.isRunning {
		return
	}

	w.isRunning = true
	go w.loop()
}

// Stop signals the worker to stop
func (w *RTSPWorker) Stop() {
	if !w.isRunning {
		return
	}

	w.isRunning = false
	close(w.stopChan)
}

// loop is the main processing loop
func (w *RTSPWorker) loop() {
	defer func() {
		w.manager.SetRunLock(w.streamID, false)
		log.Printf("[%s] RTSP worker stopped", w.streamID)
	}()

	// Main worker loop
	for w.isRunning {
		log.Printf("[%s] Stream connecting to %s", w.streamID, w.url)

		err := w.processStream()
		if err != nil {
			log.Printf("[%s] Stream error: %v", w.streamID, err)
		}

		// Check if we should continue or exit (for on-demand streams)
		if w.onDemand && !w.manager.HasViewer(w.streamID) {
			log.Printf("[%s] On-demand stream stopping: no viewers", w.streamID)
			break
		}

		// Wait before reconnecting
		select {
		case <-w.stopChan:
			return
		case <-time.After(w.reconnectTime):
			// Continue to reconnect
		}
	}
}

// processStream handles the RTSP connection and stream processing
func (w *RTSPWorker) processStream() error {
	// Timeouts for various conditions
	const (
		keyFrameTimeout    = 20 * time.Second
		clientCheckTimeout = 20 * time.Second
		dialTimeout        = 5 * time.Second
		readWriteTimeout   = 5 * time.Second
	)

	keyFrameTimer := time.NewTimer(keyFrameTimeout)
	clientCheckTimer := time.NewTimer(clientCheckTimeout)
	defer func() {
		keyFrameTimer.Stop()
		clientCheckTimer.Stop()
	}()

	// Initialize segment processing variables
	var prevKeyFrameTS time.Duration
	var segmentBuffer []*av.Packet

	// Connect to RTSP source
	client, err := rtspv2.Dial(rtspv2.RTSPClientOptions{
		URL:              w.url,
		DisableAudio:     false,
		DialTimeout:      dialTimeout,
		ReadWriteTimeout: readWriteTimeout,
		Debug:            false,
	})

	if err != nil {
		return err
	}
	defer client.Close()

	// Update codec information
	if client.CodecData != nil {
		w.manager.UpdateCodecs(w.streamID, client.CodecData)
	}

	// Determine if stream is audio-only
	var isAudioOnly bool
	if len(client.CodecData) == 1 && client.CodecData[0].Type().IsAudio() {
		isAudioOnly = true
		log.Printf("[%s] Detected audio-only stream", w.streamID)
	}

	// Main packet processing loop
	for {
		select {
		case <-w.stopChan:
			log.Printf("[%s] Stop signal received", w.streamID)
			return nil

		case <-clientCheckTimer.C:
			// For on-demand streams, check if we still have viewers
			if w.onDemand && !w.manager.HasViewer(w.streamID) {
				return configs.ErrStreamExitNoViewer
			}
			clientCheckTimer.Reset(clientCheckTimeout)

		case <-keyFrameTimer.C:
			// If we haven't received a keyframe for too long, reconnect
			return configs.ErrStreamExitNoVideoOnStream

		case signal := <-client.Signals:
			switch signal {
			case rtspv2.SignalCodecUpdate:
				log.Printf("[%s] Codec update received", w.streamID)
				w.manager.UpdateCodecs(w.streamID, client.CodecData)
			case rtspv2.SignalStreamRTPStop:
				return configs.ErrStreamExitRtspDisconnect
			}

		case packet, ok := <-client.OutgoingPacketQueue:
			if !ok {
				return configs.ErrStreamExitRtspDisconnect
			}

			// Process packet for HLS segmentation and broadcast
			if packet.IsKeyFrame || isAudioOnly {
				// Reset keyframe timeout
				keyFrameTimer.Reset(keyFrameTimeout)

				// If we already have a segment, finalize it
				if prevKeyFrameTS > 0 && len(segmentBuffer) > 0 {
					segmentDuration := packet.Time - prevKeyFrameTS
					err := w.manager.AddHLSSegment(w.streamID, segmentBuffer, segmentDuration)
					if err != nil {
						log.Printf("[%s] Error adding HLS segment: %v", w.streamID, err)
					}
					// Clear segment buffer for new segment
					segmentBuffer = make([]*av.Packet, 0, len(segmentBuffer))
				}

				// Start new segment
				prevKeyFrameTS = packet.Time
			}

			// Add packet to current segment buffer
			pktCopy := packet
			segmentBuffer = append(segmentBuffer, pktCopy)

			// Broadcast packet to all connected clients
			w.manager.BroadcastPacket(w.streamID, *packet)
		}
	}
}
