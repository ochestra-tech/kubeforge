# kubeforge

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

# k8s-cluster-setup

This is the batch script version of the go cluster created tool - KubeForge

Make it executable setup-kubernetes.sh: chmod +x setup-kubernetes.sh
Run it as root: sudo ./setup-kubernetes.sh
First run the script on the machine you want to be the master node
When prompted, indicate it's a master node
Save the join command that is generated
Run the script on each worker node
When prompted, indicate it's not a master node
Run the join command you saved earlier on each worker node
```
