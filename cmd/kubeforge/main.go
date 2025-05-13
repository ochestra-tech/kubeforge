package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ochestra-tech/kubeforge/internal/logger"
	"github.com/ochestra-tech/kubeforge/pkg/container"
	"github.com/ochestra-tech/kubeforge/pkg/distro"
	"github.com/ochestra-tech/kubeforge/pkg/kubernetes"
	"github.com/ochestra-tech/kubeforge/pkg/network"
	"github.com/ochestra-tech/kubeforge/pkg/system"
	"github.com/ochestra-tech/kubeforge/pkg/util"
)

const (
	AppName = "KubeForge"
	Version = "1.0.0"
)

func main() {
	// Initialize logger
	log := logger.New()

	// Display welcome banner
	util.DisplayBanner(AppName, Version)

	// Check if running as root
	if !system.CheckRoot() {
		log.Error("This script must be run as root")
		os.Exit(1)
	}

	// Detect Linux distribution
	dist, err := distro.Detect()
	if err != nil {
		log.Error("Error detecting distribution: %v", err)
		os.Exit(1)
	}

	log.Info("Detected Linux distribution: %s %s", dist.Name, dist.Version)

	// Perform installation steps
	if err := system.UpdateSystem(dist, log); err != nil {
		log.Error("Failed to update system: %v", err)
		os.Exit(1)
	}

	if err := system.InstallDependencies(dist, log); err != nil {
		log.Error("Failed to install dependencies: %v", err)
		os.Exit(1)
	}

	if err := system.DisableSwap(log); err != nil {
		log.Error("Failed to disable swap: %v", err)
		os.Exit(1)
	}

	if err := system.ConfigureSystem(log); err != nil {
		log.Error("Failed to configure system: %v", err)
		os.Exit(1)
	}

	if err := container.InstallContainerd(dist, log); err != nil {
		log.Error("Failed to install containerd: %v", err)
		os.Exit(1)
	}

	if err := kubernetes.Install(dist, log); err != nil {
		log.Error("Failed to install Kubernetes components: %v", err)
		os.Exit(1)
	}

	// Determine if this is a control plane node
	isControlPlane := util.PromptYesNo("Is this a control plane (master) node?")

	// Create Kubernetes configuration
	kubeConfig := kubernetes.DefaultConfig()
	kubeConfig.IsControlPlane = isControlPlane

	if isControlPlane {
		// Get configuration parameters
		defaultIP := util.GetDefaultIP()
		kubeConfig.PodCIDR = util.PromptWithDefault("Enter Pod Network CIDR", kubeConfig.PodCIDR)
		kubeConfig.ServiceCIDR = util.PromptWithDefault("Enter Service CIDR", kubeConfig.ServiceCIDR)
		kubeConfig.APIServerAddr = util.PromptWithDefault("Enter API Server Advertise Address", defaultIP)
		kubeConfig.ClusterName = util.PromptWithDefault("Enter Cluster Name", kubeConfig.ClusterName)

		// Check if HA setup is needed
		kubeConfig.HighAvailability = util.PromptYesNo("Is this a high availability setup?")
		if kubeConfig.HighAvailability {
			kubeConfig.ControlPlaneEndpoint = util.PromptWithDefault(
				"Enter control plane endpoint (DNS/IP:port)",
				fmt.Sprintf("%s:6443", kubeConfig.APIServerAddr))
		}

		// Initialize control plane
		if err := kubernetes.InitControlPlane(kubeConfig, log); err != nil {
			log.Error("Failed to initialize control plane: %v", err)
			os.Exit(1)
		}

		// Install Calico network plugin
		networkConfig := network.DefaultConfig()
		networkConfig.PodCIDR = kubeConfig.PodCIDR

		// Check if a network plugin is already installed
		existingPlugin, err := network.GetCurrentPlugin(log)
		if err == nil {
			log.Info("Detected existing network plugin: %s", existingPlugin)
			if !util.PromptYesNo("Network plugin already installed. Proceed with reinstallation?") {
				log.Info("Skipping network plugin installation")
				// Skip network installation
			} else {
				log.Info("Reinstalling network plugin...")

				// Ask user which network plugin to use
				pluginOptions := []string{"Calico", "Flannel", "Weave", "Cilium"}
				fmt.Println("Available network plugins:")
				for i, plugin := range pluginOptions {
					fmt.Printf("%d. %s\n", i+1, plugin)
				}

				selectedPlugin := util.PromptWithDefault("Select network plugin (1-4)", "1")
				pluginIndex, _ := strconv.Atoi(selectedPlugin)

				if pluginIndex >= 1 && pluginIndex <= len(pluginOptions) {
					pluginName := pluginOptions[pluginIndex-1]
					networkConfig.Plugin = network.Plugin(strings.ToLower(pluginName))

					// If Calico is selected, offer additional configuration options
					if networkConfig.Plugin == network.Calico {
						enableEncryption := util.PromptYesNo("Enable WireGuard encryption?")
						networkConfig.EnableEncryption = enableEncryption
					}

					// Install the selected network plugin
					if err := network.InstallPlugin(networkConfig, log); err != nil {
						log.Error("Failed to install %s network plugin: %v", networkConfig.Plugin, err)
						os.Exit(1)
					}
				} else {
					log.Error("Invalid selection, defaulting to Calico")
					networkConfig.Plugin = network.Calico
					if err := network.InstallPlugin(networkConfig, log); err != nil {
						log.Error("Failed to install Calico network plugin: %v", err)
						os.Exit(1)
					}
				}
			}
		}

		if util.PromptYesNo("Test network connectivity?") {
			log.Info("Testing network connectivity between pods...")
			if err := network.CheckNetworkConnectivity(log); err != nil {
				log.Warn("Network connectivity test failed: %v", err)
				if util.PromptYesNo("Continue despite network test failure?") {
					log.Info("Continuing with installation...")
				} else {
					os.Exit(1)
				}
			} else {
				log.Info("Network connectivity test successful!")
			}
		}

		// Generate join command
		joinCommand, err := kubernetes.GenerateJoinCommand(log)
		if err != nil {
			log.Error("Failed to generate join command: %v", err)
		} else {
			fmt.Println(util.ColorBlue + "Worker node join command:" + util.ColorReset)
			fmt.Println(util.ColorYellow + joinCommand + util.ColorReset)
			fmt.Println(util.ColorBlue + "Save this command to run on your worker nodes." + util.ColorReset)
		}

		// Ask about installing Kubernetes Dashboard
		installDashboard := util.PromptYesNo("Do you want to install Kubernetes Dashboard?")
		if installDashboard {
			if err := kubernetes.InstallDashboard(log); err != nil {
				log.Error("Failed to install Kubernetes Dashboard: %v", err)
			}
		}

		// Check cluster status
		kubernetes.CheckClusterStatus(log)

		log.Info("Control plane node setup complete!")
		log.Info("Your Kubernetes cluster is now operational.")
		log.Info("Install required tools on your local machine and use: kubectl cluster-info")

	} else {
		// Worker node setup
		log.Info("Worker node setup completed.")
		log.Info("Now run the join command from the master node.")

		joinCmd := util.PromptWithDefault(
			"Enter the join command from the master node or press Enter to skip",
			"")

		if joinCmd != "" {
			if err := kubernetes.JoinCluster(joinCmd, log); err != nil {
				log.Error("Failed to join the cluster: %v", err)
				os.Exit(1)
			}
		} else {
			log.Info("Join command skipped. Run the appropriate 'kubeadm join' command manually.")
		}
	}

	log.Info("Kubernetes installation completed successfully!")
}
