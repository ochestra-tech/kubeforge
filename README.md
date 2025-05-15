# KubeForge

KubeForge is a Go-based tool for automating Kubernetes cluster installation and setup on Linux systems. It provides a simple, interactive way to bootstrap both control plane and worker nodes.

## Features

- Automatic detection of Linux distribution
- Support for Debian and RedHat based distributions
- Containerd runtime installation and configuration
- Kubernetes control plane initialization
- Calico network plugin installation
- Optional Kubernetes Dashboard installation
- Worker node join command generation
- High-availability cluster setup
- Node joining (both worker and control plane nodes)
- Node labeling and tainting
- Cluster upgrade capabilities
- Cluster status checking
- robust configuration options

## Control Plane Setup

When setting up a control plane node, KubeForge will:

- Install and configure containerd
- Install Kubernetes components (kubeadm, kubelet, kubectl)
- Initialize the Kubernetes control plane
- Install the Calico network plugin
- Generate a join command for worker nodes
- Optionally install the Kubernetes Dashboard

## Worker Node Setup

When setting up a worker node, KubeForge will:

- Install and configure containerd
- Install Kubernetes components
- Prompt for the join command from the control plane
- Join the node to the cluster

## High Availability Setup

KubeForge supports high availability setups with multiple control plane nodes. When configuring a high availability cluster:

- Set up a load balancer in front of the API servers
- Configure the first control plane node with the load balancer endpoint
- Join additional control plane nodes using certificate key

## Installation

### Build from source

```bash
git clone https://github.com/ochestra-tech/kubeforge.git
cd KubeForge
make build
sudo make install
```

## Building and Installing KubeForge with Docker

This is more practical approach that uses Docker only as a build environment, producing a standalone binary that can be run directly on the host system. This addresses security concerns while leveraging containerization for consistent builds.

### Using Docker as a Build Environment

KubeForge provides a containerized build environment that produces standalone binaries for direct use on the host system. This approach avoids running privileged containers while still leveraging Docker for consistent builds.

### Key Benefits of This Approach

- Secure: No need for privileged containers at runtime

- Portable: Builds binaries for multiple architectures

- Consistent: Same build environment regardless of host OS

- Simple: Easy installation process using generated script

- Flexible: Can run directly on the host with full access to system resources

- CI/CD friendly: Easy to integrate into build pipelines

This approach gives you the best of both worlds: the consistency of containerized builds with the security and performance of running natively on the host.

a. **Build the binaries**:

```bash
./scripts/docker-build.sh
```

b. **Install KubeForge on the host system**:

```bash
./scripts/install-host.sh
```

c. **Run KubeForge**

```bash
kubeforge
```

## Manual Installation
If you prefer to build and install KubeForge manually:

1. Build the binary:
```bash
go build -o kubeforge cmd/kubeforge/main.go
```
2. Install it
```bash
sudo cp kubeforge /usr/local/bin/
sudo mkdir -p /usr/local/lib/kubeforge/assets
sudo cp -r assets/* /usr/local/lib/kubeforge/assets/
sudo chmod +x /usr/local/bin/kubeforge
```

## Usage Demo
```bash
# Build the binaries
./build.sh

# Install KubeForge on the host system
./install-host.sh

# Run KubeForge
kubeforge
```


# K8s Cluster Setup with Batch Script

scripts/k8s-cluster-setup is the batch script version of this go cluster creation tool

## Usage Instructions

1. Save the script to a file (e.g., setup-kubernetes.sh)
2. Make it executable: chmod +x setup-kubernetes.sh
3. Run it as root: sudo ./setup-kubernetes.sh
4. First run the script on the machine you want to be the master node
5. When prompted, indicate it's a master node
6. Save the join command that is generated
7. Run the script on each worker node
8. When prompted, indicate it's not a master node
9. Run the join command you saved earlier on each worker node

After completing these steps, you'll have a functional Kubernetes cluster with networking configured and ready to deploy applications.
