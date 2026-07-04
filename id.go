package studio

import (
	"fmt"
	"time"
)

func newRunID() string {
	return fmt.Sprintf("run-%d", time.Now().UnixNano())
}

func newSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
