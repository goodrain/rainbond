FROM goodrainapps/alpine:3.4

COPY acp_mq /run/acp_mq
RUN chmod 655 /run/acp_mq
EXPOSE 6300

ENV RELEASE_DESC=__RELEASE_DESC__

CMD  ["/run/acp_mq"]
