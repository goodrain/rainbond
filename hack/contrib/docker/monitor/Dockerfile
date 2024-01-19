FROM prom/prometheus:v2.20.0
ARG RELEASE_DESC
USER root
VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=${RELEASE_DESC}

COPY rainbond-monitor /run/rainbond-monitor

RUN if [ $(arch) = "arm64" ] || [ $(arch) = "aarch64" ]; then \
        wget --no-check-certificate https://pkg.goodrain.com/pkg/tini/v0.19.0/tini-static-arm64 -O /bin/tini && chmod +x /bin/tini; \
    else \
        wget --no-check-certificate https://pkg.goodrain.com/pkg/tini/v0.19.0/tini-static-amd64 -O /bin/tini && chmod +x /bin/tini; \
    fi

ADD entrypoint.sh /run/entrypoint.sh

ENTRYPOINT ["/run/entrypoint.sh"]