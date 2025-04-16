package configs

import "errors"

// 모든 에러를 한곳에서 정의
var (
	ErrStreamNotFound             = errors.New("stream not found")
	ErrStreamAlreadyExists        = errors.New("stream already exists")
	ErrStreamChannelAlreadyExists = errors.New("stream channel already exists")
	ErrStreamNotHLSSegments       = errors.New("stream hls not ts seq found")
	ErrStreamNoVideo              = errors.New("stream no video")
	ErrStreamNoClients            = errors.New("stream no clients")
	ErrStreamRestart              = errors.New("stream restart")
	ErrStreamStopCoreSignal       = errors.New("stream stop core signal")
	ErrStreamStopRTSPSignal       = errors.New("stream stop rtsp signal")
	ErrStreamChannelNotFound      = errors.New("stream channel not found")
	ErrStreamChannelCodecNotFound = errors.New("stream channel codec not ready, possible stream offline")
	ErrStreamsLen0                = errors.New("streams len zero")
	ErrStreamExitNoVideoOnStream  = errors.New("stream exit no video on stream")
	ErrStreamExitRtspDisconnect   = errors.New("stream exit rtsp disconnect")
	ErrStreamExitNoViewer         = errors.New("stream exit on demand no viewer")
)
