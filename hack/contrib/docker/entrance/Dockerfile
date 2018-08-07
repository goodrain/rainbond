FROM goodrainapps/alpine:3.4

ADD rainbond-entrance /run/rainbond-entrance
ADD entrypoint.sh /run/entrypoint.sh
VOLUME [ "/etc/goodrain" ]

ENV RELEASE_DESC=__RELEASE_DESC__

EXPOSE 8080

ENTRYPOINT ["/run/entrypoint.sh"]

