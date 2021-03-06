package server

import (
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	serverhelper "github.com/elastic/beats/metricbeat/helper/server"
	"github.com/elastic/beats/metricbeat/helper/server/tcp"
	"github.com/elastic/beats/metricbeat/helper/server/udp"
	"github.com/elastic/beats/metricbeat/mb"
)

// init registers the MetricSet with the central registry.
// The New method will be called after the setup of the module and before starting to fetch data
func init() {
	if err := mb.Registry.AddMetricSet("graphite", "server", New); err != nil {
		panic(err)
	}
}

// MetricSet type defines all fields of the MetricSet
// As a minimum it must inherit the mb.BaseMetricSet fields, but can be extended with
// additional entries. These variables can be used to persist data or configuration between
// multiple fetch calls.
type MetricSet struct {
	mb.BaseMetricSet
	server    serverhelper.Server
	processor *metricProcessor
}

// New create a new instance of the MetricSet
// Part of new is also setting up the configuration by processing additional
// configuration entries if needed.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	cfgwarn.Experimental("The graphite server metricset is experimental")

	config := DefaultGraphiteCollectorConfig()
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	var s serverhelper.Server
	var err error
	if config.Protocol == "tcp" {
		s, err = tcp.NewTcpServer(base)
	} else {
		s, err = udp.NewUdpServer(base)
	}

	if err != nil {
		return nil, err
	}

	processor := NewMetricProcessor(config.Templates, config.DefaultTemplate)

	return &MetricSet{
		BaseMetricSet: base,
		server:        s,
		processor:     processor,
	}, nil
}

// Run method provides the Graphite server with a reporter with which events can be reported.
func (m *MetricSet) Run(reporter mb.PushReporter) {
	// Start event watcher
	m.server.Start()

	for {
		select {
		case <-reporter.Done():
			m.server.Stop()
			return
		case msg := <-m.server.GetEvents():
			input := msg.GetEvent()
			bytesRaw, ok := input[serverhelper.EventDataKey]
			if ok {
				bytes, ok := bytesRaw.([]byte)
				if ok && len(bytes) != 0 {
					event, err := m.processor.Process(string(bytes))
					if err != nil {
						reporter.Error(err)
					} else {
						reporter.Event(event)
					}
				}
			}

		}
	}
}
