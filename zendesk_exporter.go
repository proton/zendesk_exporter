package main

import (
	"net/http"
	"os"
	"sync"
	"time"

	zendesk "github.com/tagnard/zendesk-go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

type ZendeskCollector struct {
	mutex  sync.RWMutex
	client *zendesk.Client

	zendeskTicketCount *prometheus.Desc
	zendeskQueueTime   *prometheus.Desc
}

func newZendeskCollector() *ZendeskCollector {
	return &ZendeskCollector{
		zendeskTicketCount: prometheus.NewDesc("zendesk_tickets_count",
			"Zendesk ticket count", nil, nil,
		),
		zendeskQueueTime: prometheus.NewDesc("zendesk_queue_time",
			"Zendesk queue time", nil, nil,
		),
	}
}

func (collector *ZendeskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.zendeskTicketCount
	ch <- collector.zendeskQueueTime
}

func (collector *ZendeskCollector) Collect(ch chan<- prometheus.Metric) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()

	client := zendesk.FromEnv(
		zendesk.LoadConfigurationFromEnv(),
	)

	var tickets []zendesk.Ticket
	var err error

	if len(os.Getenv("ZENDESK_QUERY")) != 0 {
		tickets, err = client.Ticket().Search(os.Getenv("ZENDESK_QUERY"))
		if err != nil {
			log.Error(err)
		}
	} else {
		tickets, err = client.Ticket().GetAll()
		if err != nil {
			log.Error(err)
		}
	}

	var queueTime float64

	for _, t := range tickets {
		ua, err := time.Parse(time.RFC3339, t.UpdatedAt)
		if err != nil {
			log.Error(err)
		}

		if time.Since(ua).Seconds() > queueTime {
			queueTime = time.Since(ua).Seconds()
		}
	}

	ch <- prometheus.MustNewConstMetric(collector.zendeskTicketCount, prometheus.CounterValue, float64(len(tickets)))
	ch <- prometheus.MustNewConstMetric(collector.zendeskQueueTime, prometheus.CounterValue, queueTime)
}

func main() {
	// Create a new instance of the ZendeskCollector and
	// registre it with the prometheus client
	zendeskCollector := newZendeskCollector()
	prometheus.MustRegister(zendeskCollector)

	// Start the HTTP server and expose
	// any metrics on the /metrics endpoint.
	http.Handle("/metrics", promhttp.Handler())
	log.Info("Beginning to serve on port :9802")
	log.Fatal(http.ListenAndServe(":9802", nil))
}
