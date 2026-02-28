package platform

// Platform abstracts all OS-specific operations.
type Platform interface {
	// Network
	EnableIPForwarding() error
	DisableIPForwarding() error
	IsIPForwardingEnabled() (bool, error)
	DisableFirewallInterference() error
	ClearFirewallRules() error
	DetectDefaultInterface() (string, error)
	DetectInterfaceIP(iface string) (string, error)
	DetectGateway() (string, error)
	DetectTUNInterface() (string, error)

	// Process
	FindBinary() (string, error)
	IsRunning() (bool, int, error)
	StartProcess(binary, dataDir, logFile string) (int, error)
	StopProcess() error

	// Service
	InstallService(cfg ServiceConfig) error
	UninstallService() error
}

// ServiceConfig holds information needed to create an OS-level service.
type ServiceConfig struct {
	BinaryPath string
	DataDir    string
	ConfigFile string
	LogDir     string
	WorkDir    string
}
