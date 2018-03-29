FROM 192.168.18.250:5002/library/busybox:1.26.2

# COPY launcher files.
COPY script /root/script
COPY kube-launcher /root

# SET env
RUN export PATH=$PATH:/root
RUN mkdir -p /var/log/
EXPOSE 8886
# ENV MODULE_VERSION #MODULE_VERSION#
WORKDIR /root

# RUNNING command
CMD ["COPY images/kube-launcher"]
