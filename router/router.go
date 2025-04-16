package router

import (
	"github.com/gin-gonic/gin"
	"org.donghyuns.com/rtsphls/lib"
)

func PlayRouter(server *gin.Engine) {
	server.GET("/play/hls/:cctvId/index.m3u8", lib.PlayHLS)
	server.GET("/play/hls/:suuid/segment/:seq/file.ts", lib.PlayHLSTS)
}
