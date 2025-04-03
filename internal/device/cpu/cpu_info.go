package cpu

// CPUInfo contains information about a CPU
type CPUInfo struct {
	Processor  int
	VendorID   string
	CPUFamily  string
	Model      string
	ModelName  string
	Stepping   string
	Microcode  string
	CPUMHz     float64
	CacheSize  string
	PhysicalID string
	CoreID     string
	Cores      uint
}

type CPUInfoList []CPUInfo

// CPUInfoProvider provides CPU information
type CPUInfoProvider interface {
	// CPUInfo returns a list of CPUInfo
	CPUInfo() (CPUInfoList, error)
}

type cpuInfoProvider struct{}

func (c *cpuInfoProvider) CPUInfo() (CPUInfoList, error) {
	return nil, nil
}

func NewCPUInfoProvider() *cpuInfoProvider {
	return &cpuInfoProvider{}
}
