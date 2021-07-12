FROM ibuildthecloud/suc:latest

# We modify install.sh to copy rancher-system-agent to /usr/local/bin/ because `/usr/bin` is 
# read-only in RancherOSv2
COPY install.sh /opt/rancher-system-agent-suc/
RUN wget https://github.com/rancher/system-agent/releases/download/v0.0.1-alpha35/rancher-system-agent-amd64 -O /opt/rancher-system-agent-suc/rancher-system-agent
RUN chmod +x /opt/rancher-system-agent-suc/rancher-system-agent
