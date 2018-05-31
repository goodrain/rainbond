FROM prom/prometheus

VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=__RELEASE_DESC__

ADD /rainbond-monitor /usr/bin/monitor

ENTRYPOINT /usr/bin/monitor