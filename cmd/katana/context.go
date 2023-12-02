package main

import (
	"go.uber.org/zap"
)

// appContext is the context of the Katana CLI application.
type appContext struct {
	logger *zap.Logger
}
