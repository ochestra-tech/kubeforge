package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ochestra-tech/kubeforge/internal/logger"
)

// Plugin represents a Kubernetes network plugin
type Plugin string

// Supported network plugins
const (
	Calico  Plugin = "calico"
	Flannel Plugin = "flannel"
	Weave   Plugin = "weave"
	Cilium  Plugin = "cilium"
)

// Config holds the network plugin configuration options
type Config struct {
	Plugin               Plugin
	PodCIDR              string
	MTU                  int
	IPIPMode             string // Used for Calico
	VXLANMode            string // Used for Calico and Flannel
	EnableEncryption     bool
	EnableNATOutgoing    bool
	BlockSize            int    // Used for Calico
	EnableeBPF           bool   // Used for Cilium
	KubeProxyReplacement string // Used for Cilium
	CustomValues         map[string]string
}

// DefaultConfig returns a default network configuration
func DefaultConfig() *Config {
	return &Config{
		Plugin:               Calico,
		PodCIDR:              "10.244.0.0/16",
		MTU:                  0, // Auto-detect
		IPIPMode:             "Always",
		VXLANMode:            "CrossSubnet",
		EnableEncryption:     false,
		EnableNATOutgoing:    true,
		BlockSize:            26,       // Default Calico blockSize
		EnableeBPF:           false,    // For Cilium
		KubeProxyReplacement: "strict", // For Cilium
		CustomValues:         make(map[string]string),
	}
}

// ValidateCIDR checks if the provided CIDR is valid
func ValidateCIDR(cidr string) error {
	if !strings.Contains(cidr, "/") {
		return fmt.Errorf("invalid CIDR format: %s, should be in format x.x.x.x/y", cidr)
	}

	// Additional validation could be added here
	return nil
}

// InstallPlugin installs the specified network plugin
func InstallPlugin(config *Config, log *logger.Logger) error {
	log.Info("Installing %s network plugin...", config.Plugin)

	switch config.Plugin {
	case Calico:
		return installCalico(config, log)
	case Flannel:
		return installFlannel(config, log)
	case Weave:
		return installWeave(config, log)
	case Cilium:
		return installCilium(config, log)
	default:
		return fmt.Errorf("unsupported network plugin: %s", config.Plugin)
	}
}

// installCalico installs and configures Calico
func installCalico(config *Config, log *logger.Logger) error {
	log.Info("Installing Calico network plugin...")

	// Validate CIDR
	if err := ValidateCIDR(config.PodCIDR); err != nil {
		return err
	}

	// Deploy Calico operator
	log.Info("Deploying Calico operator...")
	tigeraCmd := exec.Command("kubectl", "create", "-f",
		"https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/tigera-operator.yaml")
	tigeraCmd.Stdout = os.Stdout
	tigeraCmd.Stderr = os.Stderr

	err := tigeraCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install Tigera operator: %v", err)
	}

	// Create custom resources file for Calico
	encapsulation := "IPIP"
	if config.IPIPMode == "Never" {
		encapsulation = "None"
	}
	if config.VXLANMode != "Never" {
		encapsulation = "VXLAN" + config.VXLANMode
	}

	natOutgoing := "Enabled"
	if !config.EnableNATOutgoing {
		natOutgoing = "Disabled"
	}

	mtuValue := ""
	if config.MTU > 0 {
		mtuValue = fmt.Sprintf("mtu: %d", config.MTU)
	}

	calicoResources := fmt.Sprintf(`apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    ipPools:
    - blockSize: %d
      cidr: %s
      encapsulation: %s
      natOutgoing: %s
      nodeSelector: all()
`, config.BlockSize, config.PodCIDR, encapsulation, natOutgoing)

	// Add MTU if specified
	if mtuValue != "" {
		calicoResources += fmt.Sprintf("    %s\n", mtuValue)
	}

	// Add encryption if enabled
	if config.EnableEncryption {
		calicoResources += "    ipipMode: Always\n"
		calicoResources += "    encryption: WireGuard\n"
	}

	// Add custom values
	for key, value := range config.CustomValues {
		calicoResources += fmt.Sprintf("    %s: %s\n", key, value)
	}

	calicoResourcesPath := "/tmp/calico-custom-resources.yaml"
	err = os.WriteFile(calicoResourcesPath, []byte(calicoResources), 0644)
	if err != nil {
		return fmt.Errorf("failed to write Calico resources file: %v", err)
	}

	// Apply custom resources
	log.Info("Applying Calico custom resources...")
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
	if err := waitForPodsReady("k8s-app=calico-node", 5*time.Minute, log); err != nil {
		log.Warn("Timed out waiting for Calico pods: %v", err)
		log.Warn("Installation may still be in progress")
		return nil
	}

	log.Info("Calico network plugin successfully installed!")
	return nil
}

// installFlannel installs and configures Flannel
func installFlannel(config *Config, log *logger.Logger) error {
	log.Info("Installing Flannel network plugin...")

	// Validate CIDR
	if err := ValidateCIDR(config.PodCIDR); err != nil {
		return err
	}

	// Create flannel configuration
	flannelYaml := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: kube-flannel
  labels:
    pod-security.kubernetes.io/enforce: privileged
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: flannel
  namespace: kube-flannel
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: flannel
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: flannel
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: flannel
subjects:
- kind: ServiceAccount
  name: flannel
  namespace: kube-flannel
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-flannel-cfg
  namespace: kube-flannel
data:
  cni-conf.json: |
    {
      "name": "cbr0",
      "cniVersion": "0.3.1",
      "plugins": [
        {
          "type": "flannel",
          "delegate": {
            "hairpinMode": true,
            "isDefaultGateway": true
          }
        },
        {
          "type": "portmap",
          "capabilities": {
            "portMappings": true
          }
        }
      ]
    }
  net-conf.json: |
    {
      "Network": "%s",
      "Backend": {
        "Type": "vxlan"
      }
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-flannel-ds
  namespace: kube-flannel
spec:
  selector:
    matchLabels:
      app: flannel
  template:
    metadata:
      labels:
        app: flannel
    spec:
      serviceAccountName: flannel
      containers:
      - name: kube-flannel
        image: docker.io/flannel/flannel:v0.21.4
        command:
        - /opt/bin/flanneld
        args:
        - --ip-masq
        - --kube-subnet-mgr
`, config.PodCIDR)

	// Add MTU if specified
	if config.MTU > 0 {
		flannelYaml += fmt.Sprintf("        - --iface-mtu=%d\n", config.MTU)
	}

	// Add rest of the DaemonSet spec
	flannelYaml += `        resources:
          limits:
            cpu: 100m
            memory: 50Mi
          requests:
            cpu: 100m
            memory: 50Mi
        securityContext:
          privileged: true
        volumeMounts:
        - name: run
          mountPath: /run/flannel
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      volumes:
        - name: run
          hostPath:
            path: /run/flannel
        - name: flannel-cfg
          configMap:
            name: kube-flannel-cfg
      hostNetwork: true
      tolerations:
      - operator: Exists
      nodeSelector:
        kubernetes.io/os: linux
`

	// Write Flannel configuration to file
	flannelYamlPath := "/tmp/kube-flannel.yaml"
	err := os.WriteFile(flannelYamlPath, []byte(flannelYaml), 0644)
	if err != nil {
		return fmt.Errorf("failed to write Flannel config file: %v", err)
	}

	// Apply Flannel configuration
	log.Info("Applying Flannel configuration...")
	flannelCmd := exec.Command("kubectl", "apply", "-f", flannelYamlPath)
	flannelCmd.Stdout = os.Stdout
	flannelCmd.Stderr = os.Stderr

	if err := flannelCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply Flannel configuration: %v", err)
	}

	// Wait for Flannel pods to be ready
	log.Info("Waiting for Flannel pods to be ready...")
	if err := waitForPodsReady("app=flannel", 5*time.Minute, log); err != nil {
		log.Warn("Timed out waiting for Flannel pods: %v", err)
		log.Warn("Installation may still be in progress")
		return nil
	}

	log.Info("Flannel network plugin successfully installed!")
	return nil
}

// installWeave installs and configures Weave Net
func installWeave(config *Config, log *logger.Logger) error {
	log.Info("Installing Weave network plugin...")

	// Build Weave installation command
	weaveCmd := exec.Command("kubectl", "apply", "-f", "https://github.com/weaveworks/weave/releases/download/v2.8.1/weave-daemonset-k8s-1.11.yaml")

	// If a custom CIDR is specified, set the environment variable
	if config.PodCIDR != "" {
		if err := ValidateCIDR(config.PodCIDR); err != nil {
			return err
		}
		weaveCmd.Env = append(os.Environ(), fmt.Sprintf("IPALLOC_RANGE=%s", config.PodCIDR))
	}

	// Execute the command
	weaveCmd.Stdout = os.Stdout
	weaveCmd.Stderr = os.Stderr

	if err := weaveCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Weave Net: %v", err)
	}

	// Wait for Weave pods to be ready
	log.Info("Waiting for Weave pods to be ready...")
	if err := waitForPodsReady("name=weave-net", 5*time.Minute, log); err != nil {
		log.Warn("Timed out waiting for Weave pods: %v", err)
		log.Warn("Installation may still be in progress")
		return nil
	}

	log.Info("Weave network plugin successfully installed!")
	return nil
}

// installCilium installs and configures Cilium
func installCilium(config *Config, log *logger.Logger) error {
	log.Info("Installing Cilium network plugin...")

	// Check if Helm is installed
	helmCheckCmd := exec.Command("helm", "version", "--short")
	if err := helmCheckCmd.Run(); err != nil {
		// Install Helm if not available
		log.Info("Helm not found, installing...")

		// Get latest Helm install script
		getHelmCmd := exec.Command("sh", "-c",
			"curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash")
		getHelmCmd.Stdout = os.Stdout
		getHelmCmd.Stderr = os.Stderr

		if err := getHelmCmd.Run(); err != nil {
			return fmt.Errorf("failed to install Helm: %v", err)
		}
	}

	// Add Cilium Helm repository
	log.Info("Adding Cilium Helm repository...")
	addRepoCmd := exec.Command("helm", "repo", "add", "cilium", "https://helm.cilium.io/")
	addRepoCmd.Stdout = os.Stdout
	addRepoCmd.Stderr = os.Stderr

	if err := addRepoCmd.Run(); err != nil {
		return fmt.Errorf("failed to add Cilium Helm repository: %v", err)
	}

	// Update Helm repositories
	updateRepoCmd := exec.Command("helm", "repo", "update")
	updateRepoCmd.Run()

	// Prepare Cilium Helm install command
	helmArgs := []string{
		"install", "cilium", "cilium/cilium",
		"--namespace", "kube-system",
		"--set", fmt.Sprintf("ipam.operator.clusterPoolIPv4PodCIDR=%s", config.PodCIDR),
	}

	// Add optional configurations
	if config.MTU > 0 {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("mtu=%d", config.MTU))
	}

	if config.EnableeBPF {
		helmArgs = append(helmArgs, "--set", "bpf.masquerade=true")
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("kubeProxyReplacement=%s", config.KubeProxyReplacement))
	}

	// Add encryption if enabled
	if config.EnableEncryption {
		helmArgs = append(helmArgs, "--set", "encryption.enabled=true")
		helmArgs = append(helmArgs, "--set", "encryption.type=wireguard")
	}

	// Add custom values
	for key, value := range config.CustomValues {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("%s=%s", key, value))
	}

	// Install Cilium
	log.Info("Installing Cilium with Helm...")
	ciliumCmd := exec.Command("helm", helmArgs...)
	ciliumCmd.Stdout = os.Stdout
	ciliumCmd.Stderr = os.Stderr

	if err := ciliumCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Cilium: %v", err)
	}

	// Wait for Cilium pods to be ready
	log.Info("Waiting for Cilium pods to be ready...")
	if err := waitForPodsReady("k8s-app=cilium", 5*time.Minute, log); err != nil {
		log.Warn("Timed out waiting for Cilium pods: %v", err)
		log.Warn("Installation may still be in progress")
		return nil
	}

	log.Info("Cilium network plugin successfully installed!")
	return nil
}

// waitForPodsReady waits for pods matching the labelSelector to be ready
func waitForPodsReady(labelSelector string, timeout time.Duration, log *logger.Logger) error {
	start := time.Now()

	// Poll until pods are running
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for pods with selector %s", labelSelector)
		}

		cmd := exec.Command("kubectl", "get", "pods", "-l", labelSelector, "--all-namespaces",
			"-o", "jsonpath={.items[*].status.phase}")
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

			// If we have at least one pod and all are running, we're good
			if podsStatus != "" && allRunning {
				return nil
			}
		}

		log.Info("Waiting for pods to be ready... (%d seconds elapsed)", int(time.Since(start).Seconds()))
		time.Sleep(10 * time.Second)
	}
}

// CheckNetworkConnectivity verifies pod-to-pod connectivity
func CheckNetworkConnectivity(log *logger.Logger) error {
	log.Info("Checking network connectivity between pods...")

	// Create a test namespace
	testNamespace := "network-test-" + fmt.Sprintf("%d", time.Now().Unix())
	createNsCmd := exec.Command("kubectl", "create", "namespace", testNamespace)
	if err := createNsCmd.Run(); err != nil {
		return fmt.Errorf("failed to create test namespace: %v", err)
	}

	// Ensure namespace is deleted at the end
	defer func() {
		deleteNsCmd := exec.Command("kubectl", "delete", "namespace", testNamespace)
		deleteNsCmd.Run()
	}()

	// Create test pods
	log.Info("Creating test pods...")

	// Create first test pod
	pod1Yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: network-test-1
  namespace: %s
spec:
  containers:
  - name: network-test
    image: busybox:stable
    command: ['sh', '-c', 'sleep 3600']
`, testNamespace)

	pod1Path := "/tmp/network-test-1.yaml"
	os.WriteFile(pod1Path, []byte(pod1Yaml), 0644)

	createPod1Cmd := exec.Command("kubectl", "apply", "-f", pod1Path)
	if err := createPod1Cmd.Run(); err != nil {
		return fmt.Errorf("failed to create first test pod: %v", err)
	}

	// Create second test pod
	pod2Yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: network-test-2
  namespace: %s
spec:
  containers:
  - name: network-test
    image: busybox:stable
    command: ['sh', '-c', 'sleep 3600']
`, testNamespace)

	pod2Path := "/tmp/network-test-2.yaml"
	os.WriteFile(pod2Path, []byte(pod2Yaml), 0644)

	createPod2Cmd := exec.Command("kubectl", "apply", "-f", pod2Path)
	if err := createPod2Cmd.Run(); err != nil {
		return fmt.Errorf("failed to create second test pod: %v", err)
	}

	// Wait for pods to be ready
	log.Info("Waiting for test pods to be ready...")
	if err := waitForPodsReady(fmt.Sprintf("name in (network-test-1, network-test-2)"), 2*time.Minute, log); err != nil {
		return fmt.Errorf("test pods not ready: %v", err)
	}

	// Get IP of the second pod
	log.Info("Testing connectivity between pods...")
	podIPCmd := exec.Command("kubectl", "get", "pod", "network-test-2", "-n", testNamespace,
		"-o", "jsonpath={.status.podIP}")
	podIPOutput, err := podIPCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get pod IP: %v", err)
	}

	podIP := strings.TrimSpace(string(podIPOutput))
	if podIP == "" {
		return fmt.Errorf("could not get pod IP")
	}

	// Test connectivity from the first pod to the second pod
	pingCmd := exec.Command("kubectl", "exec", "network-test-1", "-n", testNamespace, "--",
		"ping", "-c", "3", podIP)
	pingCmd.Stdout = os.Stdout
	pingCmd.Stderr = os.Stderr

	if err := pingCmd.Run(); err != nil {
		return fmt.Errorf("connectivity test failed: %v", err)
	}

	log.Info("Network connectivity test successful!")
	return nil
}

// GetCurrentPlugin attempts to detect the currently installed network plugin
func GetCurrentPlugin(log *logger.Logger) (Plugin, error) {
	log.Info("Detecting current network plugin...")

	// Check for Calico
	calicoCmd := exec.Command("kubectl", "get", "pods", "-l", "k8s-app=calico-node", "--all-namespaces")
	if calicoCmd.Run() == nil {
		return Calico, nil
	}

	// Check for Flannel
	flannelCmd := exec.Command("kubectl", "get", "pods", "-l", "app=flannel", "--all-namespaces")
	if flannelCmd.Run() == nil {
		return Flannel, nil
	}

	// Check for Weave
	weaveCmd := exec.Command("kubectl", "get", "pods", "-l", "name=weave-net", "--all-namespaces")
	if weaveCmd.Run() == nil {
		return Weave, nil
	}

	// Check for Cilium
	ciliumCmd := exec.Command("kubectl", "get", "pods", "-l", "k8s-app=cilium", "--all-namespaces")
	if ciliumCmd.Run() == nil {
		return Cilium, nil
	}

	return "", fmt.Errorf("could not detect network plugin")
}

// GetCalicoVersion returns the installed Calico version
func GetCalicoVersion(log *logger.Logger) (string, error) {
	cmd := exec.Command("kubectl", "get", "pods", "-l", "k8s-app=calico-node", "-n", "kube-system",
		"-o", "jsonpath={.items[0].spec.containers[0].image}")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Calico version: %v", err)
	}

	// Extract version from image tag
	image := string(output)
	parts := strings.Split(image, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("could not parse Calico version from image: %s", image)
	}

	return parts[1], nil
}
