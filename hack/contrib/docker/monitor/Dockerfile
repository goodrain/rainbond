FROM rainbond/prometheus:v2.20.0

USER root
VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=__RELEASE_DESC__

COPY rainbond-monitor /run/rainbond-monitor

ADD entrypoint.sh /run/entrypoint.sh

ENTRYPOINT ["/run/entrypoint.sh"]