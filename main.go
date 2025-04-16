package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"org.donghyuns.com/rtsphls/configs"
)

func main() {
	if loadErr := godotenv.Load(".env"); loadErr != nil {
		panic(loadErr)
	}

	configs.SetGlobalConfig()
	configs.SetDatabaseConfig()

	// 종료 시그널 핸들링
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// 서버 시작
	go func() {
		log.Println("Starting server on port", manager.Server.HTTPPort)
		if err := server.Run(":" + manager.Server.HTTPPort); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 종료 시그널 대기
	<-signalChan
	log.Println("Shutting down server...")

	// lib.SaveToHls(GlobalConfig.Url)
	// lib.Start(GlobalConfig.RtspUrl)

	// lib.Start(GlobalConfig.RtspUrl)
}
