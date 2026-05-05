package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	jobsProcessed *prometheus.CounterVec
	jobsFailed    *prometheus.CounterVec
	jobDuration   *prometheus.HistogramVec
	activeJobs    prometheus.Gauge
	gatherer      prometheus.Gatherer
}

func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		jobsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_jobs_completed_total",
			Help: "Total number of successfully completed jobs.",
		}, []string{"job_type"}),
		jobsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "worker_jobs_failed_total",
			Help: "Total number of failed jobs.",
		}, []string{"job_type"}),
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "worker_job_duration_seconds",
			Help:    "Job processing duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"job_type"}),
		activeJobs: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "worker_active_jobs",
			Help: "Number of jobs currently being processed.",
		}),
	}

	reg.MustRegister(m.jobsProcessed, m.jobsFailed, m.jobDuration, m.activeJobs)
	if g, ok := reg.(prometheus.Gatherer); ok {
		m.gatherer = g
	} else {
		m.gatherer = prometheus.DefaultGatherer
	}
	return m
}

func (m *Metrics) JobCompleted(jobType string, durationSecs float64) {
	m.jobsProcessed.WithLabelValues(jobType).Inc()
	m.jobDuration.WithLabelValues(jobType).Observe(durationSecs)
}

func (m *Metrics) JobFailed(jobType string) {
	m.jobsFailed.WithLabelValues(jobType).Inc()
}

func (m *Metrics) JobStarted() {
	m.activeJobs.Inc()
}

func (m *Metrics) JobFinished() {
	m.activeJobs.Dec()
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.gatherer, promhttp.HandlerOpts{})
}
