package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"math/cmplx"
	"net/http"
	"os"
	"strconv"

	"github.com/mjibson/go-dsp/fft"
	"github.com/wcharczuk/go-chart"
)

const (
	Port        = ":3030"
	Iterations  = 100
	Start       = 0.1
	DefaultRate = "3.5"
)

var tmpl *template.Template

type context struct {
	Rate string
	Body template.HTML
}

func init() {
	var err error
	tmpl, err = template.ParseFiles("./template.html")
	if err != nil {
		panic(err)
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get(w, r)
		default:
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/chart", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getChart(w, r)
		default:
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	http.ListenAndServe(Port, nil)
}

func get(w http.ResponseWriter, r *http.Request) {
	// response is always JSON
	w.Header().Set("Content-Type", "application/json")

	rate, err := getRate(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timeSeries := logisticMap(rate)
	frequencySeries := frequencyTransform(timeSeries)

	output, err := json.Marshal(struct {
		Time      [Iterations]float64 `json:"time"`
		Frequency [Iterations]float64 `json:"frequency"`
	}{timeSeries, frequencySeries})

	// return server error if marshaling fails
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	w.Write(output)
}

func getChart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	rate, err := getRate(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timeSeries := logisticMap(rate)
	timeYSeries := timeSeries[:]

	timeXSeries := make([]float64, Iterations, Iterations)
	timeStep := 1.0 / Iterations
	for i := 0; i < Iterations; i++ {
		timeXSeries[i] = float64(i) * timeStep
	}

	timeChart := chart.Chart{
		Width:  400,
		Height: 300,
		XAxis: chart.XAxis{
			Style: chart.Style{
				Show: true,
			},
			TickPosition: chart.TickPositionBetweenTicks,
			ValueFormatter: func(v interface{}) string {
				typed := v.(float64) // O.o TODO: handle errors
				return fmt.Sprintf("%.2f", typed)
			},
		},
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show: true,
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: timeXSeries,
				YValues: timeYSeries,
			},
		},
	}

	frequencySeries := frequencyTransform(timeSeries)
	frequencyYSeries := frequencySeries[:]

	frequencyXSeries := make([]float64, Iterations, Iterations)
	frequencyStep := 0.5 / Iterations
	for i := 0; i < Iterations; i++ {
		frequencyXSeries[i] = float64(i) * frequencyStep
	}

	frequencyChart := chart.Chart{
		Width:  400,
		Height: 300,
		XAxis: chart.XAxis{
			Style: chart.Style{
				Show: true,
			},
			TickPosition: chart.TickPositionBetweenTicks,
			ValueFormatter: func(v interface{}) string {
				typed := v.(float64) // O.o TODO: handle errors
				return fmt.Sprintf("%.2f", typed)
			},
		},
		YAxis: chart.YAxis{
			Style: chart.Style{
				Show: true,
			},
		},
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: frequencyXSeries,
				YValues: frequencyYSeries,
			},
		},
	}

	var buf bytes.Buffer
	timeChart.Render(chart.SVG, &buf)
	frequencyChart.Render(chart.SVG, &buf)

	err = tmpl.Execute(w, context{
		Body: template.HTML(buf.String()),
		Rate: fmt.Sprintf("%.2f", rate),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
}

func getRate(r *http.Request) (float64, error) {
	// extract "rate" param
	rateParam := r.URL.Query().Get("rate")
	if rateParam == "" {
		rateParam = DefaultRate
	}

	// parse "rate" param or return client error
	return strconv.ParseFloat(rateParam, 64)
}

// generates the logistic map series for the given growth rate
func logisticMap(rate float64) (series [Iterations]float64) {
	// 0 < x < 1 | x(n+1) = rate * x(n) * (1 - x(n))
	x := Start
	for i := 0; i < Iterations; i++ {
		x = rate * x * (1 - x)
		series[i] = x
	}
	return
}

// transforms an array of amplitude values over time to an array of amplitude
// values, sorted by frequency
func frequencyTransform(series [Iterations]float64) (output [Iterations]float64) {
	// convert array of real numbers to array of complex numbers with no
	// imaginary component
	input := make([]complex128, Iterations, Iterations)
	for i := range series {
		input[i] = cmplx.Rect(series[i], 0)
	}

	// outsource the actual transform to fft library
	frequencies := fft.FFT(input)

	// convert array of complex numbers to array of real numbers by stripping the
	// imaginary component
	for i := range frequencies {
		output[i] = real(frequencies[i])
	}
	return
}
