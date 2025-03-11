package benchmark

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"time"
)

// Stat stores the statistics data for benchmarking results
type Stat struct {
	Data   []float64
	Size   int
	Mean   float64
	Min    float64
	Max    float64
	Median float64
	P95    float64
	P99    float64
	P999   float64
}

// WriteFile writes stat to new file in path
func (s Stat) WriteFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range s.Data {
		fmt.Fprintln(w, line)
	}
	return w.Flush()
}

func (s Stat) String() string {
	return fmt.Sprintf("size = %d\nmean = %f\nmin = %f\nmax = %f\nmedian = %f\np95 = %f\np99 = %f\np999 = %f\n", s.Size, s.Mean, s.Min, s.Max, s.Median, s.P95, s.P99, s.P999)
}

// Statistic function creates Stat object from raw latency data
func Statistic(latency []time.Duration) Stat {
	if len(latency) == 0 {
		return Stat{
			Data:   []float64{},
			Size:   0,
			Mean:   0,
			Min:    0,
			Max:    0,
			Median: 0,
			P95:    0,
			P99:    0,
			P999:   0,
		}
	}

	ms := make([]float64, len(latency))
	for i, l := range latency {
		ms[i] = float64(l.Nanoseconds()) / 1000000.0
	}
	sort.Float64s(ms)

	sum := 0.0
	for _, m := range ms {
		sum += m
	}
	size := len(ms)

	var median float64
	mid := size / 2
	if size%2 == 0 {
		median = (ms[mid-1] + ms[mid]) / 2
	} else {
		median = ms[mid]
	}

	return Stat{
		Data:   ms,
		Size:   size,
		Mean:   sum / float64(size),
		Min:    ms[0],
		Max:    ms[size-1],
		Median: median,
		P95:    percentile(ms, 0.95),
		P99:    percentile(ms, 0.99),
		P999:   percentile(ms, 0.999),
	}
}

// Helper function to calculate percentile
func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	index := int(p * float64(len(data)))
	if index < 0 {
		return data[0]
	}
	if index >= len(data) {
		return data[len(data)-1]
	}
	return data[index]
}