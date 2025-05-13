package kubernetes

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochestra-tech/kubeforge/internal/logger"
	"github.com/ochestra-tech/kubeforge/pkg/distro"
)

// Config represents Kubernetes configuration parameters
type Config struct {
	PodCIDR              string
	ServiceCIDR          string
	APIServerAddr        string
	IsControlPlane       bool
	InstallDashboard     bool
	ClusterName          string
	KubernetesVersion    string
	HighAvailability     bool
	ControlPlaneEndpoint string
	NodeName             string
	Labels               map[string]string
	Taints               []string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		PodCIDR:           "10.244.0.0/16",
		ServiceCIDR:       "10.96.0.0/12",
		APIServerAddr:     "", // Will be set dynamically
		IsControlPlane:    false,
		InstallDashboard:  false,
		ClusterName:       "kubeforge-cluster",
		KubernetesVersion: "", // Will use latest available
		HighAvailability:  false,
		NodeName:          "", // Will be set to hostname by default
		Labels:            make(map[string]string),
		Taints:            []string{},
	}
}

// Install installs Kubernetes components
func Install(dist *distro.Distribution, log *logger.Logger) error {
	log.Info("Installing Kubernetes components...")

	switch dist.Type {
	case distro.Debian:
		// Add Kubernetes apt repository
		keyCmd := exec.Command("sh", "-c",
			"curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg")
		err := keyCmd.Run()
		if err != nil {
			return err
		}

		repoCmd := exec.Command("sh", "-c",
			`echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list > /dev/null`)
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

		// Install Kubernetes components
		installCmd := exec.Command("apt-get", "install", "-y", "kubelet", "kubeadm", "kubectl")
		err = installCmd.Run()
		if err != nil {
			return err
		}

		// Hold packages to prevent automatic updates
		holdCmd := exec.Command("apt-mark", "hold", "kubelet", "kubeadm", "kubectl")
		err = holdCmd.Run()
		if err != nil {
			return err
		}

	case distro.RedHat:
		// Add Kubernetes yum repository
		repoContent := `[kubernetes]
		name=Kubernetes
		baseurl=https://pkgs.k8s.io/core:/stable:/v1.29/rpm/
		enabled=1
		gpgcheck=1
		gpgkey=https://pkgs.k8s.io/core:/stable:/v1.29/rpm/repodata/repomd.xml.key
`
		err := os.WriteFile("/etc/yum.repos.d/kubernetes.repo", []byte(repoContent), 0644)
		if err != nil {
			return err
		}

		// Install Kubernetes components
		installCmd := exec.Command("yum", "install", "-y", "kubelet", "kubeadm", "kubectl")
		err = installCmd.Run()
		if err != nil {
			return err
		}

		// Enable kubelet service
		enableCmd := exec.Command("systemctl", "enable", "kubelet")
		err = enableCmd.Run()
		if err != nil {
			return err
		}

		// SELinux settings recommended for Kubernetes on RHEL/CentOS
		selinuxCmd := exec.Command("setenforce", "0")
		selinuxCmd.Run() // Ignore errors as it might already be disabled

		// Update SELinux config file to make the change permanent
		selinuxConfig := "/etc/selinux/config"
		if _, err := os.Stat(selinuxConfig); err == nil {
			sedCmd := exec.Command("sed", "-i", "s/^SELINUX=enforcing$/SELINUX=permissive/", selinuxConfig)
			sedCmd.Run() // Ignore errors
		}

		// RHEL-specific: Enable required services for network bridge
		if dist.Name == "rhel" || dist.Name == "centos" {
			bridgeCmd := exec.Command("modprobe", "br_netfilter")
			bridgeCmd.Run()

			// Ensure bridge-nf-call-iptables is set to 1
			sysctlCmd := exec.Command("sh", "-c", "echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables")
			sysctlCmd.Run()
		}

		// RHEL 8+ and CentOS 8+ specific: Ensure legacy iptables
		majorVersion := 0
		if len(dist.Version) > 0 {
			fmt.Sscanf(dist.Version, "%d", &majorVersion)
		}

		if (dist.Name == "rhel" || dist.Name == "centos") && majorVersion >= 8 {
			// Ensure legacy iptables
			alternativesCmd := exec.Command("alternatives", "--set", "iptables", "/usr/sbin/iptables-legacy")
			alternativesCmd.Run() // Ignore errors

			// Do the same for ip6tables
			ip6tablesCmd := exec.Command("alternatives", "--set", "ip6tables", "/usr/sbin/ip6tables-legacy")
			ip6tablesCmd.Run() // Ignore errors
		}

	default:
		return fmt.Errorf("unsupported distribution for Kubernetes installation")
	}

	// Start and enable kubelet
	startCmd := exec.Command("systemctl", "enable", "kubelet")
	err := startCmd.Run()
	if err != nil {
		return err
	}

	startCmd = exec.Command("systemctl", "start", "kubelet")
	return startCmd.Run()
}

// InitControlPlane initializes the Kubernetes control plane
func InitControlPlane(config *Config, log *logger.Logger) error {
	log.Info("Initializing Kubernetes control plane node...")

	// Create kubeadm config file for more control
	if config.NodeName == "" {
		// Get hostname if not specified
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("failed to get hostname: %v", err)
		}
		config.NodeName = hostname
	}

	// Build kubeadm configuration
	kubeadmConfig := fmt.Sprintf(`apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
nodeRegistration:
  name: %s
  taints: []
localAPIEndpoint:
  advertiseAddress: %s
  bindPort: 6443
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
clusterName: %s
networking:
  podSubnet: %s
  serviceSubnet: %s
`, config.NodeName, config.APIServerAddr, config.ClusterName, config.PodCIDR, config.ServiceCIDR)

	// Add HA configuration if enabled
	if config.HighAvailability && config.ControlPlaneEndpoint != "" {
		kubeadmConfig += fmt.Sprintf("controlPlaneEndpoint: %s\n", config.ControlPlaneEndpoint)
	}

	// Add specific Kubernetes version if specified
	if config.KubernetesVersion != "" {
		kubeadmConfig += fmt.Sprintf("kubernetesVersion: %s\n", config.KubernetesVersion)
	}

	// Write config to file
	kubeadmConfigPath := "/tmp/kubeadm-config.yaml"
	err := os.WriteFile(kubeadmConfigPath, []byte(kubeadmConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write kubeadm config: %v", err)
	}

	// Initialize the cluster with the config file
	initCmd := exec.Command("kubeadm", "init", "--config", kubeadmConfigPath, "--upload-certs")

	// Redirect output to stdout/stderr
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr

	// Run the command
	err = initCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to initialize control plane: %v", err)
	}

	// Set up kubectl configuration
	log.Info("Setting up kubectl configuration...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	kubeDir := filepath.Join(homeDir, ".kube")
	err = os.MkdirAll(kubeDir, 0755)
	if err != nil {
		return err
	}

	cpCmd := exec.Command("cp", "-i", "/etc/kubernetes/admin.conf", filepath.Join(kubeDir, "config"))
	err = cpCmd.Run()
	if err != nil {
		return err
	}

	// Set proper ownership
	currentUser, err := user.Current()
	if err == nil {
		chownCmd := exec.Command("chown",
			fmt.Sprintf("%s:%s", currentUser.Uid, currentUser.Gid),
			filepath.Join(kubeDir, "config"))
		err = chownCmd.Run()
		if err != nil {
			log.Warn("Failed to set ownership on kubectl config: %v", err)
		}
	}

	// Also set up for the sudo user if running with sudo
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser != "" {
		SetupKubectlForUser(sudoUser, log)
	}

	return nil
}

// SetupKubectlForUser configures kubectl for a specific user
func SetupKubectlForUser(username string, log *logger.Logger) error {
	log.Info("Setting up kubectl for user %s", username)

	// Get user's home directory
	userHomeCmd := exec.Command("sh", "-c", fmt.Sprintf("getent passwd %s | cut -d: -f6", username))
	userHomeOutput, err := userHomeCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get home directory for user %s: %v", username, err)
	}

	userHome := strings.TrimSpace(string(userHomeOutput))
	userKubeDir := filepath.Join(userHome, ".kube")

	// Create .kube directory
	mkdirCmd := exec.Command("mkdir", "-p", userKubeDir)
	err = mkdirCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to create .kube directory for user %s: %v", username, err)
	}

	// Copy admin.conf to user's .kube directory
	cpCmd := exec.Command("cp", "-i", "/etc/kubernetes/admin.conf", filepath.Join(userKubeDir, "config"))
	err = cpCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to copy admin.conf for user %s: %v", username, err)
	}

	// Set ownership
	chownCmd := exec.Command("chown", "-R", fmt.Sprintf("%s:%s", username, username), userKubeDir)
	err = chownCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set ownership for user %s: %v", username, err)
	}

	return nil
}

// InstallCalico installs Calico network plugin
func InstallCalico(config *Config, log *logger.Logger) error {
	log.Info("Installing Calico network plugin...")

	// Deploy Calico operator
	tigeraCmd := exec.Command("kubectl", "create", "-f",
		"https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/tigera-operator.yaml")
	tigeraCmd.Stdout = os.Stdout
	tigeraCmd.Stderr = os.Stderr

	err := tigeraCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install Tigera operator: %v", err)
	}

	// Create custom resources file for Calico
	calicoResources := fmt.Sprintf(`apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    ipPools:
    - blockSize: 26
      cidr: %s
      encapsulation: VXLANCrossSubnet
      natOutgoing: Enabled
      nodeSelector: all()
`, config.PodCIDR)

	calicoResourcesPath := "/tmp/calico-custom-resources.yaml"
	err = os.WriteFile(calicoResourcesPath, []byte(calicoResources), 0644)
	if err != nil {
		return fmt.Errorf("failed to write Calico resources file: %v", err)
	}

	// Apply custom resources
	resourceCmd := exec.Command("kubectl", "create", "-f", calicoResourcesPath)
	resourceCmd.Stdout = os.Stdout
	resourceCmd.Stderr = os.Stderr

	if err := resourceCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply Calico resources: %v", err)
	}

	// Wait for Calico pods to be ready
	log.Info("Waiting for Calico pods to be ready...")

	// Give some time for the operator to start creating resources
	time.Sleep(10 * time.Second)

	// Poll until calico-node pods are running
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("kubectl", "get", "pods", "-l", "k8s-app=calico-node", "-A", "-o", "jsonpath={.items[*].status.phase}")
		output, err := cmd.Output()

		if err == nil {
			podsStatus := string(output)
			allRunning := true

			// Check if any pods are not Running
			for _, status := range strings.Fields(podsStatus) {
				if status != "Running" {
					allRunning = false
					break
				}
			}

			if podsStatus != "" && allRunning {
				log.Info("Calico network plugin successfully installed!")
				return nil
			}
		}

		log.Info("Waiting for Calico pods to be ready... (%d/%d)", i+1, maxRetries)
		time.Sleep(10 * time.Second)
	}

	log.Warn("Timed out waiting for Calico pods. Installation may still be in progress.")
	return nil
}

// InstallDashboard installs the Kubernetes Dashboard
func InstallDashboard(log *logger.Logger) error {
	log.Info("Installing Kubernetes Dashboard...")

	// Deploy dashboard
	dashboardCmd := exec.Command("kubectl", "apply", "-f",
		"https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml")
	dashboardCmd.Stdout = os.Stdout
	dashboardCmd.Stderr = os.Stderr

	err := dashboardCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install Dashboard: %v", err)
	}

	// Create admin user for Dashboard
	adminUserYaml := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: kubernetes-dashboard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: kubernetes-dashboard
`

	adminUserPath := "/tmp/dashboard-admin-user.yaml"
	err = os.WriteFile(adminUserPath, []byte(adminUserYaml), 0644)
	if err != nil {
		return fmt.Errorf("failed to write Dashboard admin user file: %v", err)
	}

	// Apply admin user config
	userCmd := exec.Command("kubectl", "apply", "-f", adminUserPath)
	userCmd.Stdout = os.Stdout
	userCmd.Stderr = os.Stderr

	if err := userCmd.Run(); err != nil {
		return fmt.Errorf("failed to create Dashboard admin user: %v", err)
	}

	// Create token for Dashboard login
	log.Info("Creating token for Dashboard login...")
	tokenCmd := exec.Command("kubectl", "-n", "kubernetes-dashboard", "create", "token", "admin-user")
	tokenOutput, err := tokenCmd.Output()

	if err != nil {
		log.Warn("Failed to create dashboard token: %v", err)
	} else {
		log.Info("Dashboard token:\n%s", string(tokenOutput))
	}

	log.Info("To access Dashboard, run: kubectl proxy")
	log.Info("Then access: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/")

	return nil
}

// GenerateJoinCommand creates a token and generates the command for worker nodes to join the cluster
func GenerateJoinCommand(log *logger.Logger) (string, error) {
	log.Info("Generating join command for worker nodes...")

	tokenCmd := exec.Command("kubeadm", "token", "create", "--print-join-command")
	output, err := tokenCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate join command: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// JoinCluster joins a worker node to an existing cluster
func JoinCluster(joinCommand string, log *logger.Logger) error {
	log.Info("Joining the Kubernetes cluster as a worker node...")

	// Execute the join command
	joinCmd := exec.Command("sh", "-c", joinCommand)
	joinCmd.Stdout = os.Stdout
	joinCmd.Stderr = os.Stderr

	if err := joinCmd.Run(); err != nil {
		return fmt.Errorf("failed to join the cluster: %v", err)
	}

	log.Info("Successfully joined the Kubernetes cluster!")
	return nil
}

// JoinControlPlane joins a node as an additional control plane node
func JoinControlPlane(joinCommand, certificateKey string, log *logger.Logger) error {
	log.Info("Joining the Kubernetes cluster as a control plane node...")

	// Add control-plane flag and certificate key
	fullJoinCommand := fmt.Sprintf("%s --control-plane --certificate-key %s", joinCommand, certificateKey)

	// Execute the join command
	joinCmd := exec.Command("sh", "-c", fullJoinCommand)
	joinCmd.Stdout = os.Stdout
	joinCmd.Stderr = os.Stderr

	if err := joinCmd.Run(); err != nil {
		return fmt.Errorf("failed to join as control plane: %v", err)
	}

	log.Info("Successfully joined as an additional control plane node!")
	return nil
}

// LabelNode adds labels to a node
func LabelNode(nodeName string, labels map[string]string, log *logger.Logger) error {
	for key, value := range labels {
		log.Info("Adding label %s=%s to node %s", key, value, nodeName)

		labelCmd := exec.Command("kubectl", "label", "nodes", nodeName, fmt.Sprintf("%s=%s", key, value))
		if err := labelCmd.Run(); err != nil {
			return fmt.Errorf("failed to add label %s=%s: %v", key, value, err)
		}
	}

	return nil
}

// TaintNode adds taints to a node
func TaintNode(nodeName string, taints []string, log *logger.Logger) error {
	for _, taint := range taints {
		log.Info("Adding taint %s to node %s", taint, nodeName)

		taintCmd := exec.Command("kubectl", "taint", "nodes", nodeName, taint)
		if err := taintCmd.Run(); err != nil {
			return fmt.Errorf("failed to add taint %s: %v", taint, err)
		}
	}

	return nil
}

// UpgradeCluster upgrades a Kubernetes cluster to a newer version
func UpgradeCluster(version string, log *logger.Logger) error {
	log.Info("Upgrading Kubernetes cluster to version %s", version)

	// Upgrade kubeadm
	log.Info("Upgrading kubeadm...")
	upgradeKubeadmCmd := exec.Command("apt-get", "update")
	upgradeKubeadmCmd.Run()

	upgradeKubeadmCmd = exec.Command("apt-get", "install", "-y", fmt.Sprintf("kubeadm=%s-*", version))
	if err := upgradeKubeadmCmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade kubeadm: %v", err)
	}

	// Plan the upgrade
	planCmd := exec.Command("kubeadm", "upgrade", "plan", version)
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	planCmd.Run() // Ignore errors, just for information

	// Apply the upgrade
	log.Info("Applying control plane upgrade...")
	applyCmd := exec.Command("kubeadm", "upgrade", "apply", version, "-y")
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr

	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade control plane: %v", err)
	}

	// Upgrade kubelet and kubectl
	log.Info("Upgrading kubelet and kubectl...")
	upgradeKubeletCmd := exec.Command("apt-get", "install", "-y",
		fmt.Sprintf("kubelet=%s-*", version),
		fmt.Sprintf("kubectl=%s-*", version))

	if err := upgradeKubeletCmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade kubelet and kubectl: %v", err)
	}

	// Restart kubelet
	restartCmd := exec.Command("systemctl", "daemon-reload")
	restartCmd.Run()

	restartCmd = exec.Command("systemctl", "restart", "kubelet")
	if err := restartCmd.Run(); err != nil {
		return fmt.Errorf("failed to restart kubelet: %v", err)
	}

	log.Info("Successfully upgraded Kubernetes control plane to version %s", version)
	log.Info("Remember to upgrade all worker nodes too!")

	return nil
}

// CheckClusterStatus checks the status of the Kubernetes cluster
func CheckClusterStatus(log *logger.Logger) error {
	log.Info("Checking Kubernetes cluster status...")

	// Check node status
	nodeCmd := exec.Command("kubectl", "get", "nodes")
	nodeCmd.Stdout = os.Stdout
	nodeCmd.Stderr = os.Stderr

	if err := nodeCmd.Run(); err != nil {
		return fmt.Errorf("failed to get nodes: %v", err)
	}

	// Check pod status across all namespaces
	podCmd := exec.Command("kubectl", "get", "pods", "--all-namespaces")
	podCmd.Stdout = os.Stdout
	podCmd.Stderr = os.Stderr

	if err := podCmd.Run(); err != nil {
		return fmt.Errorf("failed to get pods: %v", err)
	}

	// Check component status
	csCmd := exec.Command("kubectl", "get", "componentstatuses")
	csCmd.Stdout = os.Stdout
	csCmd.Stderr = os.Stderr

	if err := csCmd.Run(); err != nil {
		log.Warn("Failed to get component status: %v", err)
	}

	return nil
}
