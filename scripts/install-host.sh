#!/bin/bash

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
# This script installs KubeForge on the host system.
# It assumes that the build script has already been run and the binaries are available in the 'dist' directory.
# Check if the script is being run as root

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi  
# Check if the script is being run on a supported OS
if [[ "$OSTYPE" != "linux-gnu"* ]]; then
    echo "This script is only supported on Linux."
    exit 1
fi
# Check if the script is being run on a supported architecture
if [[ "$(uname -m)" != "x86_64" && "$(uname -m)" != "aarch64" ]]; then
    echo "This script is only supported on x86_64 and aarch64 architectures."
    exit 1
fi
# Check if the script is being run on a supported distribution
if ! grep -qE "Ubuntu|Debian|CentOS|Fedora" /etc/os-release; then
    echo "This script is only supported on Ubuntu, Debian, CentOS, and Fedora."
    exit 1
fi
# Check if the script is being run on a supported version
if ! grep -qE "20\.04|22\.04|8\.[0-9]+|9\.[0-9]+" /etc/os-release; then
    echo "This script is only supported on Ubuntu 20.04, 22.04, CentOS 8.x, and Fedora 36+."
    exit 1
fi

set -e

# Check if the dist directory exists
if [ ! -d "dist" ]; then
    echo "Error: 'dist' directory not found. Run ./build.sh first."
    exit 1
fi

# Run the install script with sudo
sudo ./dist/install.sh

echo ""
echo "KubeForge has been installed to /usr/local/bin/kubeforge"
echo "You can now run it with: kubeforge"