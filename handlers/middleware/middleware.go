package middleware

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
)

const (
	RequestLatency = "RequestLatency"
	RequestCount   = "RequestCount"
)

type LoggableHandlerFunc func(logger lager.Logger, w http.ResponseWriter, r *http.Request)

//go:generate counterfeiter -o fakes/fake_emitter.go . Emitter
type Emitter interface {
	IncrementCounter(delta int)
	UpdateLatency(latency time.Duration)
}

func LogWrap(logger, accessLogger lager.Logger, loggableHandlerFunc LoggableHandlerFunc) http.HandlerFunc {
	lagerDataFromReq := func(r *http.Request) lager.Data {
		return lager.Data{
			"method":  r.Method,
			"request": r.URL.String(),
		}
	}

	if accessLogger != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			requestLog := logger.Session("request", lagerDataFromReq(r))
			requestAccessLogger := accessLogger.Session("request", lagerDataFromReq(r))

			requestAccessLogger.Info("serving")

			requestLog.Debug("serving")

			start := time.Now()
			defer requestLog.Debug("done")
			defer func() {
				requestAccessLogger.Info("done", lager.Data{"duration": time.Since(start)})
			}()
			loggableHandlerFunc(requestLog, w, r)
		}
	} else {
		return func(w http.ResponseWriter, r *http.Request) {
			requestLog := logger.Session("request", lagerDataFromReq(r))

			requestLog.Debug("serving")
			defer requestLog.Debug("done")

			loggableHandlerFunc(requestLog, w, r)

		}
	}
}

func RecordLatency(f http.HandlerFunc, emitter Emitter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		f(w, r)
		emitter.UpdateLatency(time.Since(startTime))
	}
}

func RecordRequestCount(handler http.Handler, emitter Emitter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		emitter.IncrementCounter(1)
		handler.ServeHTTP(w, r)
	}
}
