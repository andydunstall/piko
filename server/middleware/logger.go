// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package middleware

import (
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewLogger(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path = path + "?" + c.Request.URL.RawQuery
		}

		// Process request
		c.Next()

		logger.Debug(
			"http request",
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.String("path", path),
			zap.Int64("latency", time.Since(start).Milliseconds()),
			zap.String("client-ip", c.ClientIP()),
			zap.Int("resp-size", c.Writer.Size()),
		)
	}
}
