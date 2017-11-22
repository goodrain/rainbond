FROM goodrainapps/alpine:3.4

COPY rainbond-mq /run/rainbond-mq
ADD entrypoint.sh /run/entrypoint.sh
RUN chmod 655 /run/rainbond-mq
EXPOSE 6300

ENV RELEASE_DESC=__RELEASE_DESC__

ENTRYPOINT ["/run/entrypoint.sh"]

