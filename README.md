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

## Installation

### Build from source

```bash
git clone https://github.com/ochestra-tech/kubeforge.git
cd KubeForge
make build
sudo make install

## K8s Cluster Setup with Batch Script

scripts/k8s-cluster-setup is the batch script version of this go cluster created tool

Make it executable setup-kubernetes.sh: chmod +x setup-kubernetes.sh
Run it as root: sudo ./setup-kubernetes.sh
First run the script on the machine you want to be the master node
When prompted, indicate it's a master node
Save the join command that is generated
Run the script on each worker node
When prompted, indicate it's not a master node
Run the join command you saved earlier on each worker node
```

## Building and Installing KubeForge

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
