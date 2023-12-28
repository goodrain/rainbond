FROM prom/prometheus:v2.20.0
ARG RELEASE_DESC
USER root
VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=${RELEASE_DESC}

COPY rainbond-monitor /run/rainbond-monitor

RUN if [ $(arch) = "arm64" ] || [ $(arch) = "aarch64" ]; then \
        wget https://pkg.goodrain.com/pkg/tini/v0.19.0/tini-arm -O /bin/tini && chmod +x /bin/tini; \
    else \
        wget https://pkg.goodrain.com/pkg/tini/v0.19.0/tini -O /bin/tini && chmod +x /bin/tini; \
    fi

ADD entrypoint.sh /run/entrypoint.sh

ENTRYPOINT ["/run/entrypoint.sh"]