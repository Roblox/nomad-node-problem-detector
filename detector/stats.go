package detector

import (
	"fmt"
	"math"
	"time"

	"github.com/mackerelio/go-osstat/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

// MemoryStats represents stats related to virtual memory usage
type MemoryStats struct {
	Total     uint64
	Available uint64
	Used      uint64
	Free      uint64
}

// DiskStats represents stats related to disk usage
type DiskStats struct {
	Device            string
	Mountpoint        string
	Size              uint64
	Used              uint64
	Available         uint64
	UsedPercent       float64
	InodesUsedPercent float64
}

// CPUStats represents stats related to cpu usage
type CPUStats struct {
	User   float64
	System float64
	Idle   float64
}

// Collect CPU stats
func collectCPUStats() (*CPUStats, error) {
	before, err := cpu.Get()
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Duration(1) * time.Second)
	after, err := cpu.Get()
	if err != nil {
		return nil, err
	}

	total := float64(after.Total - before.Total)
	cpuStats := &CPUStats{
		User:   (float64(after.User-before.User) / total) * 100,
		System: (float64(after.System-before.System) / total) * 100,
		Idle:   (float64(after.Idle-before.Idle) / total) * 100,
	}

	return cpuStats, nil
}

// Collect disk usage for root (/) partition
func collectDiskStats() (*DiskStats, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var diskStats *DiskStats
	for _, partition := range partitions {
		if partition.Mountpoint != "/" {
			continue
		}
		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			return nil, fmt.Errorf("Error fetching host disk usage stats: %s\n", partition.Mountpoint)
		}
		diskStats = toDiskStats(usage, &partition)
	}

	return diskStats, nil
}

// toDiskStats merges UsageStat and PartitionStat to create a DiskStat
func toDiskStats(usage *disk.UsageStat, partitionStat *disk.PartitionStat) *DiskStats {
	ds := DiskStats{
		Size:              usage.Total,
		Used:              usage.Used,
		Available:         usage.Free,
		UsedPercent:       usage.UsedPercent,
		InodesUsedPercent: usage.InodesUsedPercent,
	}
	if math.IsNaN(ds.UsedPercent) {
		ds.UsedPercent = 0.0
	}
	if math.IsNaN(ds.InodesUsedPercent) {
		ds.InodesUsedPercent = 0.0
	}

	if partitionStat != nil {
		ds.Device = partitionStat.Device
		ds.Mountpoint = partitionStat.Mountpoint
	}
	return &ds
}

// Collect memory stats.
func collectMemoryStats() (*MemoryStats, error) {
	memStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	mem := &MemoryStats{
		Total:     memStats.Total,
		Available: memStats.Available,
		Used:      memStats.Used,
		Free:      memStats.Free,
	}

	return mem, nil
}
