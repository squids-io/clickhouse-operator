package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
)

func (w *PrometheusWriter) WriteClickhouseUp(req string) {
	writeSingleMetricToPrometheus(w.out, "clickhouse_up", "status of chi", req, prometheus.GaugeValue, []string{"chi", "namespace", "hostname"},
		w.chi.Name, w.chi.Namespace, w.hostname)
}

func (f *ClickHouseFetcher) getHostnameStatus(chi *WatchedCHI) (string, error) {
	status := "1"
	trans := http.Transport {
		DisableKeepAlives : true,
	}
	client := http.Client {
		Transport : &trans,
	}
	for _, hostname := range chi.Hostnames {
		resp, err := client.Get(fmt.Sprintf("http://%s:8123/ping", hostname))
		if err != nil {
			status = "0"
			return "0", err
		}
		if resp == nil {
			status = "0"
			return "0", nil
		}
		defer resp.Body.Close()
		if resp.Status == "200 OK" {
			continue
		}
	}
	return status, nil
}

