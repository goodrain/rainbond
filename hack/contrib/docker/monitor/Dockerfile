FROM prom/prometheus:v2.2.1

VOLUME ["/prometheusdata"]

ENV RELEASE_DESC=__RELEASE_DESC__

ADD rainbond-monitor /run/monitor

ENTRYPOINT [""]
CMD [ "/run/monitor" ]