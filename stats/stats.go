package stats

import (
	"log"

	"github.com/c9s/goprocinfo/linux"
)

type Stats struct {
	MemStats  *linux.MemInfo
	DiskStats *linux.Disk
	LoadStats *linux.LoadAvg
	CpuStats  *linux.CPUStat
}

func (s *Stats) MemTotalKb() uint64 {
	return s.MemStats.MemTotal
}

func (s *Stats) MemAvailableKb() uint64 {
	return s.MemStats.MemAvailable
}

func (s *Stats) MemUsedKb() uint64 {
	return s.MemStats.MemTotal - s.MemStats.MemAvailable
}

func (s *Stats) MemeUsedPercent() uint64 {
	return s.MemStats.MemAvailable / s.MemStats.MemTotal
}

func (s *Stats) DiskTotal() uint64 {
	return s.DiskStats.All
}

func (s *Stats) DiskFree() uint64 {
	return s.DiskStats.Free
}

func (s *Stats) DiskUsed() uint64 {
	return s.DiskStats.Used
}

func (s *Stats) CpuUsage() float64 {
	idle := s.CpuStats.Idle + s.CpuStats.IOWait
	nonIdle := s.CpuStats.User + s.CpuStats.Nice + s.CpuStats.System
	total := idle + nonIdle
	if total == 0 {
		return 0
	}
	return (float64(total-idle) / float64(total))
}

func GetStats() *Stats {
	return &Stats{
		MemStats:  GetMemoryInfo(),
		DiskStats: GetDiskInfo(),
		CpuStats:  GetCpuStats(),
		LoadStats: GetLoadAvg(),
	}
}

func GetMemoryInfo() *linux.MemInfo {
	pseudoDirPath := "/proc/meminfo"
	memStats, err := linux.ReadMemInfo(pseudoDirPath)
	if err != nil {
		log.Printf("Error reading from %v, err: %v\n", pseudoDirPath, err)
		return &linux.MemInfo{}
	}
	return memStats
}

func GetDiskInfo() *linux.Disk {
	pseudoDirPath := "/"
	diskStats, err := linux.ReadDisk(pseudoDirPath)
	if err != nil {
		log.Printf("Error reading from disk at %v, err:%v\n", pseudoDirPath, err)
		return &linux.Disk{}
	}
	return diskStats
}

func GetCpuStats() *linux.CPUStat {
	pseudoDirPath := "/proc/stat"
	stats, err := linux.ReadStat(pseudoDirPath)
	if err != nil {
		log.Printf("Error reading from %v, err:%v\n", pseudoDirPath, err)
		return &linux.CPUStat{}
	}
	return &stats.CPUStatAll
}

func GetLoadAvg() *linux.LoadAvg {
	pseudoDirPath := "/proc/loadavg"
	loadavg, err := linux.ReadLoadAvg(pseudoDirPath)
	if err != nil {
		log.Printf("Error reading from %v, err:%v\n", pseudoDirPath, err)
		return &linux.LoadAvg{}
	}
	return loadavg
}
