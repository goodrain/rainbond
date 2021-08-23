FROM goodrainapps/alpine:3.4
ARG RELEASE_DESC
COPY . /run
RUN chmod +x /run/rainbond-grctl /run/entrypoint.sh
VOLUME [ "/rootfs/root","/rootfs/path","/ssl" ]
ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT ["/run/entrypoint.sh"]