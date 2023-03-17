#!/usr/bin/env bash
export CPU_ARCH=$(uname -m |sed -e "s/x86_64/amd64/" |sed -e "s/aarch64/arm64/")
echo $CPU_ARCH
curl -LO https://go.dev/dl/go1.18.1.linux-$CPU_ARCH.tar.gz; mkdir -p /usr/local; tar -C /usr/local -xzf go1.18.1.linux-$CPU_ARCH.tar.gz; rm -f go1.18.1.linux-$CPU_ARCH.tar.gz

echo uname -r

if [ $CPU_ARCH == "amd64" ]; then 
    echo "install bcc for amd64"
    yum install -y http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/bcc-0.24.0-2.el8.x86_64.rpm
    yum install -y http://mirror.centos.org/centos/8-stream/PowerTools/x86_64/os/Packages/bcc-devel-0.24.0-2.el8.x86_64.rpm
fi

if [ $CPU_ARCH == "s390x" ]; then 
    echo "install bcc for s390x"
    yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/s390x/os/Packages/kernel-devel-5.14.0-285.el9.s390x.rpm
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/s390x/os/Packages/bcc-0.24.0-2.el9.s390x.rpm
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/CRB/s390x/os/Packages/bcc-devel-0.24.0-2.el9.s390x.rpm
fi
	
if [ $CPU_ARCH == "arm64" ]; then 
    echo "install bcc for arm64"
    yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/aarch64/os/Packages/kernel-devel-5.14.0-285.el9.aarch64.rpm
	yum install -y https://rpmfind.net/linux/centos-stream/9-stream/AppStream/aarch64/os/Packages/bcc-0.24.0-4.el9.aarch64.rpm
    yum install -y https://rpmfind.net/linux/centos-stream/9-stream/CRB/aarch64/os/Packages/bcc-devel-0.24.0-4.el9.aarch64.rpm
fi