package stats

import (
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	snet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/wonli/aqi/ws"
)

type NetStats struct {
	LastSent uint64
	LastRecv uint64
	LastTime time.Time
}

// Stats contains statistical data on CPU and memory usage
type Stats struct {
	Timestamp    time.Time  `json:"timestamp"`
	SvrCPUUsage  float64    `json:"svrCPUUsage"`  // Overall CPU usage rate
	SvrMemoryPct float64    `json:"svrMemoryPct"` // Total memory
	LoadAverage  [2]float64 `json:"loadAverage"`  // 1, 5 minutes average load

	CPUUsage       float64 `json:"CPUUsage"`            // Current process's CPU usage rate
	MemoryUsage    float64 `json:"memoryUsage"`         // Current process's memory usage
	MemoryUsagePct float64 `json.json:"memoryUsagePct"` // Current process's memory usage percentage
	ThreadCount    int     `json:"threadCount"`         // Current process's thread count
	Goroutines     int     `json:"goroutines"`          // Number of Go coroutines
	HeapAlloc      float64 `json:"heapAlloc"`           // Memory allocated in the heap currently
	HeapSys        float64 `json:"heapSys"`             // Total heap memory obtained from the system
	HeapInuse      float64 `json:"heapInuse"`           // Heap memory in use
	HeapPct        float64 `json:"heapPct"`             // Percentage of heap memory obtained from the system

	LoginCount  int `json:"loginCount"`      // Online users
	GuestCount  int `json.json:"guestCount"` // Visitors
	Connections int `json:"connections"`     // Current process's network connections

	SentRate float64 `json:"sentRate"` // Sending rate KB/s
	RecvRate float64 `json:"recvRate"` // Receiving rate KB/s

	MaxMemoryUsage float64 `json:"maxMemoryUsage"` // Maximum memory
	MaxGoroutines  int     `json:"maxGoroutines"`  // Maximum coroutines
}

// Collector manages the collection and storage of statistical data
type Collector struct {
	mu       sync.Mutex // mu
	stats    []Stats    // Slice for storing statistical data
	netStats NetStats   // transmission rate

	capacity  int // Maximum capacity of the slice
	interval2 time.Duration
}

var capacity = 30
var once sync.Once
var collectorInstance *Collector

func InitStatsCollector() *Collector {
	once.Do(func() {
		collectorInstance = &Collector{
			stats:    make([]Stats, 0, capacity),
			capacity: capacity,
		}
	})

	return collectorInstance
}

// Collect starts collecting statistical data
func (sc *Collector) Collect(interval time.Duration) {
	go func() {
		sc.doCollect(100 * time.Millisecond)
		for {
			sc.doCollect(0)
			time.Sleep(interval)
		}
	}()
}

func (sc *Collector) doCollect(interval time.Duration) {
	currentStats := Stats{}

	if ws.Hub == nil {
		return
	}

	// User data
	currentStats.LoginCount = ws.Hub.LoginCount
	currentStats.GuestCount = ws.Hub.GuestCount

	// Get CPU usage rate
	cpuPercentages, err := cpu.Percent(interval, false)
	if err == nil && len(cpuPercentages) > 0 {
		currentStats.SvrCPUUsage = cpuPercentages[0]
	}

	// Get average load
	avgLoad, err := load.Avg()
	if err == nil {
		currentStats.LoadAverage = [2]float64{avgLoad.Load1, avgLoad.Load5}
	}

	// Get network interface statistics
	netIOCounters, err := snet.IOCounters(true)
	if err == nil {
		var totalBytesSent, totalBytesRecv uint64
		for _, counter := range netIOCounters {
			totalBytesSent += counter.BytesSent
			totalBytesRecv += counter.BytesRecv
		}

		currentTime := time.Now()
		if !sc.netStats.LastTime.IsZero() {
			// Calculate time difference (seconds)
			duration := currentTime.Sub(sc.netStats.LastTime).Seconds()

			// Calculate byte differences and convert to KB
			sentDiff := float64(totalBytesSent-sc.netStats.LastSent) / 1024.0
			recvDiff := float64(totalBytesRecv-sc.netStats.LastRecv) / 1024.0

			// Calculate rate per second
			currentStats.SentRate = sentDiff / duration
			currentStats.RecvRate = recvDiff / duration
		}

		// Update the last statistics
		sc.netStats.LastSent = totalBytesSent
		sc.netStats.LastRecv = totalBytesRecv
		sc.netStats.LastTime = currentTime
	}

	// Get memory statistics
	vmem, err := mem.VirtualMemory()
	if err == nil {
		currentStats.SvrMemoryPct = vmem.UsedPercent
	}

	// Get current process ID and process object
	pid := os.Getpid()
	proc, err := process.NewProcess(int32(pid))
	if err == nil {
		// Get current process's CPU usage rate
		procPercent, err := proc.Percent(interval)
		if err == nil {
			currentStats.CPUUsage = procPercent
		}

		// Get current process's memory statistics
		memInfo, err := proc.MemoryInfo()
		if err == nil && memInfo != nil {
			currentStats.MemoryUsage = formatMegabytes(bytesToMegabytes(memInfo.RSS)) // RSS is Resident Set Size
			vmem, err := mem.VirtualMemory()
			if err == nil {
				currentStats.MemoryUsagePct = float64(memInfo.RSS) / float64(vmem.Total) * 100
			}

			if currentStats.MemoryUsage > currentStats.MaxMemoryUsage {
				currentStats.MaxMemoryUsage = currentStats.MemoryUsage
			}
		}

		// Get current process's thread count
		threads, err := proc.NumThreads()
		if err == nil {
			currentStats.ThreadCount = int(threads)
		}
	}

	// Get current process's network connection information
	connections, err := snet.ConnectionsPid("all", int32(pid))
	if err == nil {
		currentStats.Connections = len(connections)
	}

	// Get Go runtime memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentStats.Goroutines = runtime.NumGoroutine()
	currentStats.HeapAlloc = formatMegabytes(bytesToMegabytes(memStats.HeapAlloc))
	currentStats.HeapSys = formatMegabytes(bytesToMegabytes(memStats.HeapSys))
	currentStats.HeapInuse = formatMegabytes(bytesToMegabytes(memStats.HeapInuse))
	if currentStats.Goroutines > currentStats.MaxGoroutines {
		currentStats.MaxGoroutines = currentStats.Goroutines
	}

	// Get heap memory usage
	if currentStats.HeapSys > 0 {
		currentStats.HeapPct = currentStats.HeapInuse / currentStats.HeapSys * 100
	}

	// Timestamp when statistics are completed
	currentStats.Timestamp = time.Now()

	// Lock and update statistical data
	sc.mu.Lock()
	sc.stats = append(sc.stats, currentStats)
	if len(sc.stats) > sc.capacity {
		sc.stats = sc.stats[1:]
	}
	sc.mu.Unlock()

	// Publish data
	ws.Hub.PubSub.Pub("sys:status", currentStats)
}

// GetStats returns all the collected statistical data
func (sc *Collector) GetStats() []Stats {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Return a copy of the slice to avoid external modification
	statsCopy := make([]Stats, len(sc.stats))
	copy(statsCopy, sc.stats)
	return statsCopy
}

func bytesToMegabytes(bytes uint64) float64 {
	return float64(bytes) / 1024.0 / 1024.0
}

func formatMegabytes(mb float64) float64 {
	return math.Round(mb*100) / 100
}
