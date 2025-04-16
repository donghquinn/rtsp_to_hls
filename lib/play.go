package lib

import (
	"bytes"
	"log"
	"strconv"
	"time"

	"github.com/deepch/vdk/format/ts"
	"github.com/gin-gonic/gin"
)

// PlayHLS handles m3u8 playlist requests
func PlayHLS(c *gin.Context, streamManager *StreamManager) {
	cctvId := c.Param("cctvId")

	// Check if stream exists
	if !streamManager.StreamExists(cctvId) {
		// Try to get URL from database
		streamURL, err := GetDataUrl(cctvId)
		if err != nil {
			log.Printf("Error getting stream URL for CCTV ID %s: %v", cctvId, err)
			c.String(404, "Stream not found")
			return
		}

		// Add stream to manager
		streamManager.AddStream(cctvId, streamURL, true)

		// Start RTSP worker
		worker := NewRTSPWorker(streamManager, cctvId, streamURL, true)
		worker.Start()
	} else if streamManager.RunIfNotRunning(cctvId) {
		// Start RTSP worker if not running
		stream, _ := streamManager.GetStream(cctvId)
		worker := NewRTSPWorker(streamManager, cctvId, stream.URL, stream.OnDemand)
		worker.Start()
	}

	// Wait for playlist to be ready (with timeout)
	const maxRetries = 40
	const retryInterval = 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		playlist, segmentCount, err := streamManager.GetHLSM3U8(cctvId)
		if err != nil {
			log.Printf("Error getting playlist for CCTV ID %s: %v", cctvId, err)
			c.String(500, "Error generating playlist")
			return
		}

		if segmentCount >= 2 {
			c.Header("Content-Type", "application/vnd.apple.mpegurl")
			c.Header("Cache-Control", "no-cache")
			c.String(200, playlist)
			return
		}

		if i == 0 || i == maxRetries/2 {
			log.Printf("Waiting for HLS segments for CCTV ID %s (%d/%d)", cctvId, i+1, maxRetries)
		}

		time.Sleep(retryInterval)
	}

	c.String(504, "Timeout waiting for stream to initialize")
}

// PlayHLSTS handles TS segment requests
func PlayHLSTS(c *gin.Context, streamManager *StreamManager) {
	cctvId := c.Param("cctvId")

	// Parse segment sequence number
	seqStr := c.Param("seq")
	seq, err := strconv.Atoi(seqStr)
	if err != nil {
		log.Printf("Invalid segment number: %s", seqStr)
		c.String(400, "Invalid segment number")
		return
	}

	// Get codec information
	codecs, err := streamManager.GetCodecs(cctvId)
	if err != nil {
		log.Printf("Error getting codecs for CCTV ID %s: %v", cctvId, err)
		c.String(500, "Stream codec information not available")
		return
	}

	// Get segment data
	packetData, err := streamManager.GetHLSSegment(cctvId, seq)
	if err != nil {
		log.Printf("Error getting segment %d for CCTV ID %s: %v", seq, cctvId, err)
		c.String(404, "Segment not found")
		return
	}

	if len(packetData) == 0 {
		c.String(404, "Empty segment")
		return
	}

	// Create TS muxer
	outBuffer := bytes.NewBuffer([]byte{})
	muxer := ts.NewMuxer(outBuffer)

	// Write TS header
	if err := muxer.WriteHeader(codecs); err != nil {
		log.Printf("Error writing TS header: %v", err)
		c.String(500, "Error generating segment")
		return
	}

	// Enable padding for continuous counter
	muxer.PaddingToMakeCounterCont = true

	// Write packets
	for _, packet := range packetData {
		packet.CompositionTime = 1
		if err := muxer.WritePacket(*packet); err != nil {
			log.Printf("Error writing packet to TS muxer: %v", err)
			c.String(500, "Error generating segment")
			return
		}
	}

	// Write trailer
	if err := muxer.WriteTrailer(); err != nil {
		log.Printf("Error writing TS trailer: %v", err)
		c.String(500, "Error generating segment")
		return
	}

	// Send response
	c.Header("Content-Type", "video/mp2t")
	c.Header("Cache-Control", "no-cache")
	c.Data(200, "video/mp2t", outBuffer.Bytes())
}
