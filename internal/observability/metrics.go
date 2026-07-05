package observability

import (
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds the HTTP Prometheus collectors and exposes Echo middleware.
type Metrics struct {
	registry        *prometheus.Registry
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// NewMetrics registers Go runtime + HTTP collectors on a fresh registry.
func NewMetrics() (*Metrics, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests handled, by method/route/status.",
	}, []string{"method", "route", "status"})

	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	registry.MustRegister(requestsTotal, requestDuration)

	return &Metrics{registry: registry, requestsTotal: requestsTotal, requestDuration: requestDuration}, nil
}

// Registry exposes the underlying registry for the /metrics handler.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// EchoMiddleware records request count + duration per method/route/status.
func (m *Metrics) EchoMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			status := strconv.Itoa(c.Response().Status)
			route := c.Path()
			if route == "" {
				route = c.Request().URL.Path
			}
			m.requestsTotal.WithLabelValues(c.Request().Method, route, status).Inc()
			m.requestDuration.WithLabelValues(c.Request().Method, route, status).Observe(time.Since(start).Seconds())
			return err
		}
	}
}
