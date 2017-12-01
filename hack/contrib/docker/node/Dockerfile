FROM goodrainapps/alpine:3.4

COPY rainbond-node /run/rainbond-node
ADD entrypoint.sh /run/entrypoint.sh
RUN chmod 655 /run/rainbond-node
EXPOSE 6300

ENV RELEASE_DESC=__RELEASE_DESC__

ENTRYPOINT ["/run/entrypoint.sh"]