FROM goodrainapps/alpine:3.4

COPY rainbond_mq /run/rainbond_mq
RUN chmod 655 /run/rainbond_mq
EXPOSE 6300

ENV RELEASE_DESC=__RELEASE_DESC__

CMD  ["/run/rainbond_mq"]
