package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ochestra-tech/kubeforge/internal/logger"
	"github.com/ochestra-tech/kubeforge/pkg/distro"
)

// InstallContainerd installs and configures containerd
func InstallContainerd(dist *distro.Distribution, log *logger.Logger) error {
	log.Info("Installing containerd...")

	// Download and add Docker's official GPG key
	gpgCmd := exec.Command("sh", "-c",
		fmt.Sprintf("curl -fsSL https://download.docker.com/linux/%s/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
			strings.ToLower(dist.Name)))
	err := gpgCmd.Run()
	if err != nil {
		return err
	}

	// Add Docker apt repository
	switch dist.Type {
	case distro.Debian:
		// Get codename for Debian/Ubuntu
		lsbCmd := exec.Command("lsb_release", "-cs")
		codename, err := lsbCmd.Output()
		if err != nil {
			return err
		}

		repoCmd := exec.Command("sh", "-c",
			fmt.Sprintf(`echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/%s %s stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`,
				dist.Name, strings.TrimSpace(string(codename))))
		err = repoCmd.Run()
		if err != nil {
			return err
		}

		// Update package lists
		updateCmd := exec.Command("apt-get", "update")
		err = updateCmd.Run()
		if err != nil {
			return err
		}

		// Install containerd
		installCmd := exec.Command("apt-get", "install", "-y", "containerd.io")
		err = installCmd.Run()
		if err != nil {
			return err
		}

	case distro.RedHat:
		// Add repo for CentOS/RHEL/Fedora
		repoCmd := exec.Command("yum-config-manager", "--add-repo",
			fmt.Sprintf("https://download.docker.com/linux/%s/docker-ce.repo", dist.Name))
		err = repoCmd.Run()
		if err != nil {
			return err
		}

		// Install containerd
		installCmd := exec.Command("yum", "install", "-y", "containerd.io")
		err = installCmd.Run()
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported distribution for containerd installation")
	}

	// Configure containerd
	err = os.MkdirAll("/etc/containerd", 0755)
	if err != nil {
		return err
	}

	// Generate default config
	configCmd := exec.Command("sh", "-c", "containerd config default | tee /etc/containerd/config.toml > /dev/null")
	err = configCmd.Run()
	if err != nil {
		return err
	}

	// Set systemd cgroup driver
	sedCmd := exec.Command("sed", "-i", "s/SystemdCgroup = false/SystemdCgroup = true/g", "/etc/containerd/config.toml")
	err = sedCmd.Run()
	if err != nil {
		return err
	}

	// Restart and enable containerd
	restartCmd := exec.Command("systemctl", "restart", "containerd")
	err = restartCmd.Run()
	if err != nil {
		return err
	}

	enableCmd := exec.Command("systemctl", "enable", "containerd")
	return enableCmd.Run()
}
