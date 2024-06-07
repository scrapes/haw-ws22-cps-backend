package main

import (
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"os"
)

var Logger = getLogger()

func getLogger() *zap.Logger {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stdout, zap.DebugLevel)
	logger := zap.New(core, zap.AddCaller())

	return logger
}
