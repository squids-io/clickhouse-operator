package metrics

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strings"
)

func (w *PrometheusWriter) WriteClickhouseUp(req string) {
	writeSingleMetricToPrometheus(w.out, "clickhouse_up", "status of chi", req, prometheus.GaugeValue, []string{"chi", "namespace", "hostname", "target"},
		w.chi.Name, w.chi.Namespace, w.hostname, strings.Replace(w.chi.Name, "clickhouse", "squids", -1))
}

func (f *ClickHouseFetcher) getHostnameStatus(hostname string) (string, error) {
	trans := http.Transport {
		DisableKeepAlives : true,
	}
	client := http.Client {
		Transport : &trans,
	}
	resp, err := client.Get(fmt.Sprintf("http://%s:8123/ping", hostname))
	if err != nil {
		return "0", nil
	}
	defer resp.Body.Close()
	if resp != nil &&  resp.Status == "200 OK"{
		return "1", nil
	} else {
		return "0", nil
	}
}

