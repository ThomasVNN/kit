package influx

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics3/generic"
	"github.com/go-kit/kit/metrics3/teststat"
	influxdb "github.com/influxdata/influxdb/client/v2"
)

func TestCounter(t *testing.T) {
	in := New(map[string]string{"a": "b"}, influxdb.BatchPointsConfig{}, log.NewNopLogger())
	re := regexp.MustCompile(`influx_counter,a=b count=([0-9\.]+) [0-9]+`) // reverse-engineered :\
	counter := in.NewCounter("influx_counter")
	value := func() float64 {
		client := &bufWriter{}
		in.WriteTo(client)
		match := re.FindStringSubmatch(client.buf.String())
		f, _ := strconv.ParseFloat(match[1], 64)
		return f
	}
	if err := teststat.TestCounter(counter, value); err != nil {
		t.Fatal(err)
	}
}

func TestGauge(t *testing.T) {
	in := New(map[string]string{"foo": "alpha"}, influxdb.BatchPointsConfig{}, log.NewNopLogger())
	re := regexp.MustCompile(`influx_gauge,foo=alpha value=([0-9\.]+) [0-9]+`)
	gauge := in.NewGauge("influx_gauge")
	value := func() float64 {
		client := &bufWriter{}
		in.WriteTo(client)
		match := re.FindStringSubmatch(client.buf.String())
		f, _ := strconv.ParseFloat(match[1], 64)
		return f
	}
	if err := teststat.TestGauge(gauge, value); err != nil {
		t.Fatal(err)
	}
}

func TestHistogram(t *testing.T) {
	in := New(map[string]string{"foo": "alpha"}, influxdb.BatchPointsConfig{}, log.NewNopLogger())
	re := regexp.MustCompile(`influx_histogram,foo=alpha bar="beta",value=([0-9\.]+) [0-9]+`)
	histogram := in.NewHistogram("influx_histogram").With("bar", "beta")
	quantiles := func() (float64, float64, float64, float64) {
		w := &bufWriter{}
		in.WriteTo(w)
		h := generic.NewHistogram("h", 50)
		matches := re.FindAllStringSubmatch(w.buf.String(), -1)
		for _, match := range matches {
			f, _ := strconv.ParseFloat(match[1], 64)
			h.Observe(f)
		}
		return h.Quantile(0.50), h.Quantile(0.90), h.Quantile(0.95), h.Quantile(0.99)
	}
	if err := teststat.TestHistogram(histogram, quantiles, 0.01); err != nil {
		t.Fatal(err)
	}
}

func TestHistogramLabels(t *testing.T) {
	in := New(map[string]string{}, influxdb.BatchPointsConfig{}, log.NewNopLogger())
	h := in.NewHistogram("foo")
	h.Observe(123)
	h.With("abc", "xyz").Observe(456)
	w := &bufWriter{}
	if err := in.WriteTo(w); err != nil {
		t.Fatal(err)
	}
	if want, have := 2, len(strings.Split(strings.TrimSpace(w.buf.String()), "\n")); want != have {
		t.Errorf("want %d, have %d", want, have)
	}
}

type bufWriter struct {
	buf bytes.Buffer
}

func (w *bufWriter) Write(bp influxdb.BatchPoints) error {
	for _, p := range bp.Points() {
		fmt.Fprintf(&w.buf, p.String()+"\n")
	}
	return nil
}
