package main

import (
	"go.uber.org/zap"
)

type appContext struct {
	logger *zap.Logger
}
