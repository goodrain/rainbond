FROM alpine:3

RUN apk add --no-cache \
    bash \
    bind-tools \
    busybox-extras \
    ca-certificates \
    conntrack-tools \
    curl \
    drill \
    ethtool \
    file \
    htop \
    iftop \
    iproute2 \
    iptables \
    iputils \
    jq \
    less \
    lsof \
    mtr \
    nano \
    net-tools \
    netcat-openbsd \
    nmap \
    openssh-client \
    openssl \
    procps \
    socat \
    strace \
    tcpdump \
    tini \
    tree \
    vim \
    wget \
    yq \
    && update-ca-certificates

ENV LANG=C.UTF-8

WORKDIR /root

ENTRYPOINT ["/sbin/tini", "--"]
CMD ["sleep", "infinity"]
