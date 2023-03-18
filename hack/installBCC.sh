#!/usr/bin/env bash
export CPU_ARCH=$(uname -m |sed -e "s/x86_64/amd64/" |sed -e "s/aarch64/arm64/")

if [ $CPU_ARCH == "amd64" ]; then 
    echo "install bcc for amd64"
    yum install -y http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/bcc-0.24.0-2.el8.x86_64.rpm
    yum install -y http://mirror.centos.org/centos/8-stream/PowerTools/x86_64/os/Packages/bcc-devel-0.24.0-2.el8.x86_64.rpm
    echo "for amd64 install GPU"
    dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel8/x86_64/cuda-rhel8.repo
    dnf clean all
    dnf -y module install nvidia-driver:latest-dkms
    dnf -y install cuda
fi

if [ $CPU_ARCH == "s390x" ]; then 
    echo "install bcc for s390x"
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/s390x/os/Packages/bcc-0.24.0-2.el9.s390x.rpm
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/CRB/s390x/os/Packages/bcc-devel-0.24.0-2.el9.s390x.rpm
fi
	
if [ $CPU_ARCH == "arm64" ]; then 
    echo "install bcc for arm64"
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/aarch64/os/Packages/bcc-0.24.0-4.el9.aarch64.rpm
    yum install -y https://rpmfind.net/linux/centos-stream/9-stream/CRB/aarch64/os/Packages/bcc-devel-0.24.0-4.el9.aarch64.rpm
    echo "for amd64 install GPU"
    dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel8/ppc64le/cuda-rhel8.repo
    dnf clean all
    dnf -y module install nvidia-driver:latest-dkms
    dnf -y install cuda
fi