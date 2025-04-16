package lib

import (
	"bytes"
	"log"
	"time"

	"github.com/deepch/vdk/format/ts"
	"github.com/gin-gonic/gin"
)

func PlayHLS(c *gin.Context) {
	cctvId := c.Param("cctvId")

	config := LoadConfig(cctvId)

	config.RunIFNotRun(cctvId)
	for i := 0; i < 40; i++ {
		index, seq, err := config.StreamHLSm3u8(cctvId)
		if err != nil {
			log.Println(err)
			return
		}
		if seq >= 6 {
			_, err := c.Writer.Write([]byte(index))
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
		log.Println("Play list not ready wait or try update page")
		time.Sleep(1 * time.Second)
	}
}

// PlayHLSTS send client ts segment
func PlayHLSTS(c *gin.Context) {
	cctvId := c.Param("cctvId")

	config := LoadConfig(cctvId)

	codecs := config.coGe(c.Param("cctvId"))
	if codecs == nil {
		return
	}
	outfile := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(outfile)
	err := Muxer.WriteHeader(codecs)
	if err != nil {
		log.Println(err)
		return
	}
	Muxer.PaddingToMakeCounterCont = true
	seqData, err := config.StreamHLSTS(c.Param("cctvId"), stringToInt(c.Param("seq")))
	if err != nil {
		log.Println(err)
		return
	}
	if len(seqData) == 0 {
		log.Println(err)
		return
	}
	for _, v := range seqData {
		v.CompositionTime = 1
		err = Muxer.WritePacket(*v)
		if err != nil {
			log.Println(err)
			return
		}
	}
	err = Muxer.WriteTrailer()
	if err != nil {
		log.Println(err)
		return
	}
	_, err = c.Writer.Write(outfile.Bytes())
	if err != nil {
		log.Println(err)
		return
	}
}
