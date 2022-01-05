FROM prom/prometheus:v2.20.0
ARG RELEASE_DESC
USER root
VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=${RELEASE_DESC}

COPY rainbond-monitor /run/rainbond-monitor

ADD entrypoint.sh /run/entrypoint.sh

ENTRYPOINT ["/run/entrypoint.sh"]