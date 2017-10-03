package main

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/heroku/authenticater"
	metrics "github.com/rcrowley/go-metrics"
)

type payload struct {
	SourceAddr string
	RequestID  string
	Body       []byte
	WaitCh     chan struct{}
}

func NewPayload(sa string, ri string, b []byte) payload {
	return payload{
		SourceAddr: sa,
		RequestID:  ri,
		Body:       b,
		WaitCh:     make(chan struct{}, 1),
	}
}

type FixerFunc func(io.Reader, string, string) ([]byte, error)

type httpServer struct {
	Config         IssConfig
	FixerFunc      FixerFunc
	shutdownCh     shutdownCh
	deliverer      deliverer
	isShuttingDown bool
	auth           authenticater.Authenticater
	posts          metrics.Timer   // tracks metrics about posts
	healthChecks   metrics.Timer   // tracks metrics about health checks
	pErrors        metrics.Counter // tracks the count of post errors
	pSuccesses     metrics.Counter // tracks the number of post successes
	pAuthErrors    metrics.Counter // tracks the count of auth errors
	pAuthSuccesses metrics.Counter // tracks the number of auth successes
	sync.WaitGroup
}

const (
	ctLogplexV1 = "application/logplex-1"
	ctMsgpack   = "application/msgpack"
)

func newHTTPServer(config IssConfig, auth authenticater.Authenticater, fixerFunc FixerFunc, deliverer deliverer) *httpServer {
	return &httpServer{
		auth:           auth,
		Config:         config,
		FixerFunc:      fixerFunc,
		deliverer:      deliverer,
		shutdownCh:     make(shutdownCh),
		posts:          metrics.GetOrRegisterTimer("log-iss.http.logs", config.MetricsRegistry),
		healthChecks:   metrics.GetOrRegisterTimer("log-iss.http.healthchecks", config.MetricsRegistry),
		pErrors:        metrics.GetOrRegisterCounter("log-iss.http.logs.errors", config.MetricsRegistry),
		pSuccesses:     metrics.GetOrRegisterCounter("log-iss.http.logs.successes", config.MetricsRegistry),
		pAuthErrors:    metrics.GetOrRegisterCounter("log-iss.auth.errors", config.MetricsRegistry),
		pAuthSuccesses: metrics.GetOrRegisterCounter("log-iss.auth.successes", config.MetricsRegistry),
		isShuttingDown: false,
	}
}

func (s *httpServer) handleHTTPError(w http.ResponseWriter, errMsg string, errCode int, fields ...log.Fields) {
	ff := log.Fields{"post.code": errCode}
	for _, f := range fields {
		for k, v := range f {
			ff[k] = v
		}
	}

	s.pErrors.Inc(1)
	log.WithFields(ff).Error(errMsg)
	http.Error(w, errMsg, errCode)
}

func extractRemoteAddr(r *http.Request) string {
	remoteAddr := r.Header.Get("X-Forwarded-For")
	if remoteAddr == "" {
		remoteAddrParts := strings.Split(r.RemoteAddr, ":")
		remoteAddr = strings.Join(remoteAddrParts[:len(remoteAddrParts)-1], ":")
	}
	return remoteAddr
}

func (s *httpServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	defer s.posts.UpdateSince(time.Now())

	if s.Config.EnforceSsl && r.Header.Get("X-Forwarded-Proto") != "https" {
		s.handleHTTPError(w, "Only SSL requests accepted", 400)
		return
	}

	if s.isShuttingDown {
		s.handleHTTPError(w, "Shutting down", 503)
		return
	}

	if r.Method != "POST" {
		s.handleHTTPError(w, "Only POST is accepted", 400)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !s.validContentType(contentType) {
		s.handleHTTPError(w, "Only Content-Type application/logplex-1 is accepted", 400)
		return
	}

	if !s.auth.Authenticate(r) {
		s.pAuthErrors.Inc(1)
		s.handleHTTPError(w, "Unable to authenticate request", 401)
		return
	}

	s.pAuthSuccesses.Inc(1)

	remoteAddr := extractRemoteAddr(r)
	requestID := r.Header.Get("X-Request-Id")
	logplexDrainToken := r.Header.Get("Logplex-Drain-Token")

	body := r.Body
	var err error

	if r.Header.Get("Content-Encoding") == "gzip" {
		body, err = gzip.NewReader(r.Body)
		if err != nil {
			s.handleHTTPError(w, "Could not decode gzip request", 500)
			return
		}
		defer body.Close()
	}

	if contentType == ctLogplexV1 {
		if err, status := s.process(body, remoteAddr, requestID, logplexDrainToken); err != nil {
			s.handleHTTPError(w, err.Error(), status, log.Fields{"remote_addr": remoteAddr, "requestId": requestID, "logdrain_token": logplexDrainToken})
			return
		}
		s.pSuccesses.Inc(1)
	} else if contentType == ctMsgpack {
		s.handleHTTPError(w, "Not Supported", http.StatusNotImplemented, log.Fields{"remote_addr": remoteAddr, "requestId": requestID, "logdrain_token": logplexDrainToken})
		return
	} else {
		s.handleHTTPError(w, "Not Supported", http.StatusNotImplemented, log.Fields{"remote_addr": remoteAddr, "requestId": requestID, "logdrain_token": logplexDrainToken})
		return
	}

	s.pSuccesses.Inc(1)
}

func (s *httpServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	defer s.healthChecks.UpdateSince(time.Now())
	if s.isShuttingDown {
		http.Error(w, "Shutting down", 503)
		return
	}
}

func (s *httpServer) Run() error {
	go s.awaitShutdown()

	//FXME: check outlet depth?
	http.HandleFunc("/health", s.handleHealth)
	http.HandleFunc("/logs", s.handleLogs)

	return http.ListenAndServe(":"+s.Config.HttpPort, nil)
}

func (s *httpServer) awaitShutdown() {
	<-s.shutdownCh
	s.isShuttingDown = true
	log.WithFields(log.Fields{"ns": "http", "at": "shutdown"}).Info()
}

func (s *httpServer) process(r io.Reader, remoteAddr string, requestID string, logplexDrainToken string) (error, int) {
	s.Add(1)
	defer s.Done()

	fixedBody, err := s.FixerFunc(r, remoteAddr, logplexDrainToken)
	if err != nil {
		return errors.New("Problem fixing body: " + err.Error()), http.StatusBadRequest
	}

	payload := NewPayload(remoteAddr, requestID, fixedBody)
	if err := s.deliverer.Deliver(payload); err != nil {
		return errors.New("Problem delivering body: " + err.Error()), http.StatusGatewayTimeout
	}

	return nil, 200
}

func (s *httpServer) validContentType(ct string) bool {
	var cts = []string{ctLogplexV1, ctMsgpack}

	for _, vct := range cts {
		if ct == vct {
			return true
		}
	}

	return false
}
