FROM ubuntu:24.04
ENV DEBIAN_FRONTEND=noninteractive HOME=/root
RUN apt-get update && apt-get install -y --no-install-recommends \
      dbus-daemon dbus-bin iproute2 python3 libcap2-bin \
      libglib2.0-bin gsettings-desktop-schemas dconf-service util-linux \
    && rm -rf /var/lib/apt/lists/*
CMD ["sleep", "infinity"]
