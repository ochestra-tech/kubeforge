package system

import (
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/ochestra-tech/kubeforge/internal/logger"
	"github.com/ochestra-tech/kubeforge/pkg/distro"
)

// CheckRoot returns true if the current user is root
func CheckRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0"
}

// UpdateSystem updates system packages
func UpdateSystem(dist *distro.Distribution, log *logger.Logger) error {
	log.Info("Updating system packages...")

	var cmd *exec.Cmd
	switch dist.Type {
	case distro.Debian:
		cmd = exec.Command("apt-get", "update")
		err := cmd.Run()
		if err != nil {
			return err
		}
		cmd = exec.Command("apt-get", "upgrade", "-y")
	case distro.RedHat:
		cmd = exec.Command("yum", "update", "-y")
	default:
		log.Warn("Unsupported distribution for automatic updates. Please update manually.")
		return nil
	}

	return cmd.Run()
}

// InstallDependencies installs required dependencies
func InstallDependencies(dist *distro.Distribution, log *logger.Logger) error {
	log.Info("Installing dependencies...")

	var cmd *exec.Cmd
	switch dist.Type {
	case distro.Debian:
		cmd = exec.Command("apt-get", "install", "-y",
			"apt-transport-https", "ca-certificates",
			"curl", "software-properties-common", "gnupg2")
	case distro.RedHat:
		cmd = exec.Command("yum", "install", "-y",
			"yum-utils", "device-mapper-persistent-data", "lvm2", "curl")
	default:
		log.Warn("Unsupported distribution for automatic dependency installation. Please install dependencies manually.")
		return nil
	}

	return cmd.Run()
}

// DisableSwap disables swap memory (required for Kubernetes)
func DisableSwap(log *logger.Logger) error {
	log.Info("Disabling swap...")

	// Turn off swap
	swapoffCmd := exec.Command("swapoff", "-a")
	err := swapoffCmd.Run()
	if err != nil {
		return err
	}

	// Comment out swap entries in /etc/fstab
	fstabData, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return err
	}

	lines := strings.Split(string(fstabData), "\n")
	for i, line := range lines {
		if strings.Contains(line, "swap") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			lines[i] = "# " + line
		}
	}

	return os.WriteFile("/etc/fstab", []byte(strings.Join(lines, "\n")), 0644)
}

// ConfigureSystem sets up system settings for Kubernetes
func ConfigureSystem(log *logger.Logger) error {
	log.Info("Configuring system settings for Kubernetes...")

	// Create directory if it doesn't exist
	err := os.MkdirAll("/etc/modules-load.d", 0755)
	if err != nil {
		return err
	}

	// Set up kernel modules
	kernelModules := `overlay
br_netfilter
`
	err = os.WriteFile("/etc/modules-load.d/k8s.conf", []byte(kernelModules), 0644)
	if err != nil {
		return err
	}

	// Load kernel modules
	for _, module := range []string{"overlay", "br_netfilter"} {
		cmd := exec.Command("modprobe", module)
		err := cmd.Run()
		if err != nil {
			log.Warn("Failed to load module %s: %v", module, err)
		}
	}

	// Set up sysctl parameters
	sysctlParams := `net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
`
	err = os.MkdirAll("/etc/sysctl.d", 0755)
	if err != nil {
		return err
	}

	err = os.WriteFile("/etc/sysctl.d/k8s.conf", []byte(sysctlParams), 0644)
	if err != nil {
		return err
	}

	// Apply sysctl parameters
	cmd := exec.Command("sysctl", "--system")
	return cmd.Run()
}
