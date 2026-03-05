package core

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

const statsHistorySize = 60

type SystemStats struct {
	CPUPercent   float64
	MemUsed      uint64
	MemTotal     uint64
	MemPercent   float64
	DiskUsed     uint64
	DiskTotal    uint64
	DiskPercent  float64
	LoadAvg      [3]float64
	Uptime       time.Duration
	NetBytesSent uint64
	NetBytesRecv uint64
	CPUHistory   []float64
	RAMHistory   []float64
	CollectedAt  time.Time
}

type StatsCollector struct {
	cpuHistory []float64
	ramHistory []float64
	lastNet    *net.IOCountersStat
}

func NewStatsCollector() *StatsCollector {
	return &StatsCollector{}
}

func (sc *StatsCollector) Collect() *SystemStats {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s := &SystemStats{CollectedAt: time.Now()}

	// CPU — non-blocking with 0 interval (uses diff from last call)
	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(percents) > 0 {
		s.CPUPercent = percents[0]
	}

	if v, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.MemUsed = v.Used
		s.MemTotal = v.Total
		s.MemPercent = v.UsedPercent
	}

	if d, err := disk.UsageWithContext(ctx, "/"); err == nil {
		s.DiskUsed = d.Used
		s.DiskTotal = d.Total
		s.DiskPercent = d.UsedPercent
	}

	if l, err := load.AvgWithContext(ctx); err == nil {
		s.LoadAvg = [3]float64{l.Load1, l.Load5, l.Load15}
	}

	if up, err := host.UptimeWithContext(ctx); err == nil {
		s.Uptime = time.Duration(up) * time.Second
	}

	if counters, err := net.IOCountersWithContext(ctx, false); err == nil && len(counters) > 0 {
		all := counters[0]
		if sc.lastNet != nil {
			s.NetBytesSent = all.BytesSent - sc.lastNet.BytesSent
			s.NetBytesRecv = all.BytesRecv - sc.lastNet.BytesRecv
		}
		sc.lastNet = &all
	}

	// Update history ring buffers
	sc.cpuHistory = appendRing(sc.cpuHistory, s.CPUPercent, statsHistorySize)
	sc.ramHistory = appendRing(sc.ramHistory, s.MemPercent, statsHistorySize)
	s.CPUHistory = sc.cpuHistory
	s.RAMHistory = sc.ramHistory

	return s
}

func appendRing(buf []float64, val float64, maxLen int) []float64 {
	buf = append(buf, val)
	if len(buf) > maxLen {
		buf = buf[len(buf)-maxLen:]
	}
	return buf
}
