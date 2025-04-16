package configs

import "os"

type GlobalConf struct {
	AppHost    string
	AppPort    string
	Url        string
	RtspUrl    string
	HlsRtspUrl string
}

var GlobalConfig GlobalConf

func SetGlobalConfig() {
	GlobalConfig.AppHost = os.Getenv("APP_HOST")
	GlobalConfig.AppPort = os.Getenv("APP_PORT")
	GlobalConfig.Url = os.Getenv("RTSP_URL")
	GlobalConfig.RtspUrl = os.Getenv("RTSP_PARSE_URL")
	GlobalConfig.HlsRtspUrl = os.Getenv("HLS_RTSP_URL")
}
