# Copyright 2025 The Ochestra Technologies Limited.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# Kubernetes Cluster Setup Script
/**
 * @file k8s-cluster-setup.sh
 * @brief This script sets up a Kubernetes cluster on Linux systems.
 * @version 1.0
 * @date 2023-10-01
 * 
 * @section DESCRIPTION
 * This script automates the process of setting up a Kubernetes cluster on Linux systems. It handles both master (control plane) and worker node configurations. This script should be run on all nodes (both master and worker nodes)
 * 
 * @section USAGE
 * Run this script with root privileges to set up the Kubernetes cluster.
 * 
 * @section LICENSE
 * This project is licensed under the MIT License.
 *
 * @section AUTHOR
 * Collins Oronsaye <  

 (control plane) and worker node configurations.
*/

# Initial Setup and Environment Detection

#!/bin/bash
set -e  # Exit immediately if a command fails

# Color codes and logging functions
# ...

# Check if script is run as root
if [[ $EUID -ne 0 ]]; then
    error "This script must be run as root"
    exit 1
fi

# Determine Linux distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    VERSION_ID=$VERSION_ID
else
    error "Cannot determine Linux distribution"
    exit 1
fi

# System preparation
# Update the system
log "Updating system packages..."

if [[ $OS == "ubuntu" || $OS == "debian" ]]; then
    apt update && apt upgrade -y
    apt install -y curl wget apt-transport-https ca-certificates software-properties-common
elif [[ $OS == "centos" || $OS == "rhel" || $OS == "fedora" ]]; then
    yum update -y
    yum install -y curl wget
else
    error "Unsupported Linux distribution: $OS"
    exit 1
fi

# Install necessary dependencies
# ...
log "Installing necessary dependencies..."
if [[ $OS == "ubuntu" || $OS == "debian" ]]; then
    apt install -y apt-transport-https ca-certificates curl gnupg lsb-release
elif [[ $OS == "centos" || $OS == "rhel" || $OS == "fedora" ]]; then
    yum install -y yum-utils device-mapper-persistent-data lvm2
else
    error "Unsupported Linux distribution: $OS"
    exit 1
fi

# Disable swap
log "Disabling swap..."
swapoff -a
# Comment out swap line in /etc/fstab
if [ -f /etc/fstab ]; then
    sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
fi

# KERNEL CONFIGURATION
# Load necessary kernel modules

log "Configuring system settings for Kubernetes..."

# Set up kernel modules
cat > /etc/modules-load.d/k8s.conf <<EOF
overlay
br_netfilter
EOF

modprobe overlay
modprobe br_netfilter

# Set up required sysctl parameters
cat > /etc/sysctl.d/k8s.conf <<EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF

# Apply sysctl parameters without reboot
sysctl --system

#Container Runtime Installation

# Install containerd
log "Installing containerd..."
curl -fsSL https://download.docker.com/linux/$OS/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# ... distribution-specific installation steps ...

# Configure containerd
mkdir -p /etc/containerd
containerd config default | tee /etc/containerd/config.toml > /dev/null
# Set systemd cgroup driver
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml

# Restart containerd
systemctl restart containerd
systemctl enable containerd

# Kubernetes Components Installation
log "Installing Kubernetes components (kubeadm, kubelet, kubectl)..."

# Install Kubernetes components
log "Installing Kubernetes components..."

if [[ "$OS" == "ubuntu" ]] || [[ "$OS" == "debian" ]]; then
    curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list > /dev/null
    apt-get update
    apt-get install -y kubelet kubeadm kubectl
    apt-mark hold kubelet kubeadm kubectl
elif [[ "$OS" == "centos" ]] || [[ "$OS" == "rhel" ]] || [[ "$OS" == "fedora" ]]; then
    # ... RPM-based installation ...
fi

# Start and enable kubelet
systemctl enable kubelet
systemctl start kubelet

# Kubernetes Cluster Initialization
log "Initializing Kubernetes cluster..."

# Determine if this is a master node or worker node
read -p "Is this a master node? (y/n): " IS_MASTER
if [[ "$IS_MASTER" =~ ^[Yy]$ ]]; then
    # Initialize Kubernetes cluster
    log "Initializing Kubernetes master node..."
    
    # ... Gather configuration settings ...
    
    # Initialize the cluster
    kubeadm init --pod-network-cidr=$POD_CIDR --service-cidr=$SERVICE_CIDR --apiserver-advertise-address=$API_ADDR --control-plane-endpoint=$API_ADDR
    
    # Set up kubectl for the user
    mkdir -p $HOME/.kube
    cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
    chown $(id -u):$(id -g) $HOME/.kube/config
    
    # ... Set up for regular user if exists ...

    # Install a pod network add-on (e.g., Calico, Flannel)
    log "Installing Calico network add-on..."
    
    #Network add-on installation
    # kubectl apply -f https://docs.projectcalico.org/manifests/calico.yaml
    # ... Other network add-ons can be installed similarly ...

    # Install a Pod network add-on (Calico)
    log "Installing Calico network plugin..."
    kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.27.0/manifests/tigera-operator.yaml
    
    # Create custom resources for Calico
    cat > calico-custom-resources.yaml <<EOF
    apiVersion: operator.tigera.io/v1
    kind: Installation
    metadata:
    name: default
    spec:
    calicoNetwork:
        ipPools:
        - blockSize: 26
        cidr: $POD_CIDR
        encapsulation: VXLANCrossSubnet
        natOutgoing: Enabled
        nodeSelector: all()
    EOF
    kubectl create -f calico-custom-resources.yaml

    # Join worker nodes to the cluster
    log "To join worker nodes to the cluster, run the following command on each worker node:"

    # Generate join command
    # Generate join command for worker nodes

    log "Generating join command for worker nodes..."
    JOIN_COMMAND=$(kubeadm token create --print-join-command)
    echo -e "${GREEN}Worker node join command:${NC}"
    echo -e "${YELLOW}$JOIN_COMMAND${NC}"
    echo -e "${GREEN}Save this command to run on your worker nodes.${NC}"

    # Optional Dashboard Installation

    # Install Kubernetes Dashboard (optional)
    read -p "Do you want to install Kubernetes Dashboard? (y/n): " INSTALL_DASHBOARD
    if [[ "$INSTALL_DASHBOARD" =~ ^[Yy]$ ]]; then
        log "Installing Kubernetes Dashboard..."
        kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml
        
        # Create admin user for Dashboard
        # ... yaml definition ...
        kubectl apply -f dashboard-admin-user.yaml
        
        # Get token for Dashboard login
        log "Creating token for Dashboard login..."
        kubectl -n kubernetes-dashboard create token admin-user
        
        log "To access Dashboard, run: kubectl proxy"
        log "Then access: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/"
    fi

    # Worker Node Configuration
    # If this is a worker node, join the cluster
    if [[ "$IS_MASTER" =~ ^[Nn]$ ]]; then
        log "Joining the worker node to the cluster..."
        # Use the join command generated earlier
        eval $JOIN_COMMAND
    fi
else
    # If this is a worker node, join the cluster
    log "Joining the worker node to the cluster..."
    # Use the join command generated earlier
    eval $JOIN_COMMAND
fi

    # Optional Metrics Server Installation
    read -p "Do you want to install Metrics Server? (y/n): " INSTALL_METRICS
    if [[ "$INSTALL_METRICS" =~ ^[Yy]$ ]]; then
        log "Installing Metrics Server..."
        kubectl apply -f