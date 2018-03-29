# Kubernetes自动化部署FAQ



### Centos 启动UFleet失败

```
# vi /etc/sysctl.conf
```
在文件末尾加上
```
net.bridge.bridge-nf-call-iptables=1
net.bridge.bridge-nf-call-ip6tables=1
```
然后重启系统