package rate

import (
	"time"

	appErr "internal/errors"
	"internal/logger"
	"internal/state"
)

func CheckRateLimit(s *state.State) error {
	var (
		limitSeconds *int
		lastRequest  *int64
		waitEnabled  bool
	)

	s.Read(func(st *state.State) {
		limitSeconds = st.RateLimitSeconds
		lastRequest = st.LastRequestUnixMs
		waitEnabled = st.RateLimitWait
	})

	if limitSeconds == nil {
		return nil
	}

	now := time.Now().UnixMilli()
	if lastRequest == nil {
		s.Update(func(st *state.State) {
			st.LastRequestUnixMs = &now
		})
		return nil
	}

	elapsedSeconds := float64(now-*lastRequest) / 1000.0
	if elapsedSeconds > float64(*limitSeconds) {
		s.Update(func(st *state.State) {
			st.LastRequestUnixMs = &now
		})
		return nil
	}

	waitSeconds := int64(float64(*limitSeconds) - elapsedSeconds + 0.999)

	if !waitEnabled {
		logger.Warn("Rate limit exceeded. Need to wait %d more seconds.", waitSeconds)
		resp := appErr.NewJSONResponse(429, map[string]any{"message": "Rate limit exceeded"})
		return appErr.NewHTTPError("Rate limit exceeded", resp)
	}

	logger.Warn("Rate limit reached. Waiting %d seconds before proceeding...", waitSeconds)
	time.Sleep(time.Duration(waitSeconds) * time.Second)
	logger.Info("Rate limit wait completed, proceeding with request")
	updated := time.Now().UnixMilli()
	s.Update(func(st *state.State) {
		st.LastRequestUnixMs = &updated
	})
	return nil
}
