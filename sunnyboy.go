package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

type SunnyBoyOpts struct {
	SkipVerifyCert bool
	URL            string
	logger         log.Logger
}

func NewExporter(o SunnyBoyOpts, l log.Logger) (prometheus.Collector, error) {
	s := &SunnyBoyExporter{
		insecure: o.SkipVerifyCert,
		url:      o.URL,
		l:        l,
	}
	return s, s.init()
}

type SunnyBoyExporter struct {
	url       string
	insecure  bool
	l         log.Logger
	locale    map[string]string
	metadatas map[string]metadata
	value     *prometheus.GaugeVec
	up        prometheus.Gauge
}

type metadata struct {
	TagIdEvtMsg int `json:"TagIdEvtMsg,omitempty"`
	Unit        int `json:"Unit,omitempty"`
}

type results struct {
	Result map[string]deviceResult `json:"result"`
}

type deviceResult map[string]sensorResult

type sensorResult map[string][]values

type values struct {
	Val interface{} `json:"val"`
}

func (s *SunnyBoyExporter) Collect(c chan<- prometheus.Metric) {
	g := s.gauge()
	var err error
	defer func() {
		if err != nil {
			s.up.Set(0)
			s.l.Log("err", err)
		} else {
			s.up.Set(1)
		}
		s.up.Collect(c)
	}()

	req, err := http.NewRequest("GET", s.url+"/dyn/getDashValues.json", nil)
	if err != nil {
		return
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var r results
	err = json.Unmarshal(body, &r)
	if err != nil {
		return
	}

	for dev, res := range r.Result {
		for sens, sensres := range res {
			for valid, vals := range sensres {
				for i, v := range vals {
					if metric, ok := v.Val.(float64); ok {
						metadata := s.metadatas[sens]
						name := s.locale[fmt.Sprintf("%d", metadata.TagIdEvtMsg)]
						g.WithLabelValues(dev, valid, fmt.Sprintf("%d", i), sens, name).Set(metric)
					}
				}
			}
		}
	}

	g.Collect(c)
}

func (s *SunnyBoyExporter) Describe(c chan<- *prometheus.Desc) {
	s.gauge().Describe(c)
	s.up.Describe(c)
}
func (s *SunnyBoyExporter) gauge() *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sunny_boy_value",
			Help: "Value from the Sunnyboy API",
		},
		[]string{"device", "sensor", "result", "id", "name"},
	)
}

func (s *SunnyBoyExporter) client() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

func (s *SunnyBoyExporter) init() error {
	err := s.initLocale()
	if err != nil {
		return err
	}
	err = s.initMedatatas()
	if err != nil {
		return err
	}
	s.up = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sunny_boy_up",
			Help: "Is sunny boy scrape successful",
		},
	)
	s.up.Set(0)
	return nil
}

func (s *SunnyBoyExporter) initLocale() error {
	req, err := http.NewRequest("GET", s.url+"/data/l10n/en-US.json", nil)
	if err != nil {
		return err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &s.locale)
	if err != nil {
		return err
	}
	return nil
}

func (s *SunnyBoyExporter) initMedatatas() error {
	req, err := http.NewRequest("GET", s.url+"/data/ObjectMetadata_Istl.json", nil)
	if err != nil {
		return err
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &s.metadatas)
	if err != nil {
		return err
	}
	return nil
}
