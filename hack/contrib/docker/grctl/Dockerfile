ARG BASE_IMAGE_VERSION
FROM goodrainapps/alpine:${BASE_IMAGE_VERSION}
ARG RELEASE_DESC
COPY . /run
RUN chmod +x /run/rainbond-grctl /run/entrypoint.sh
VOLUME [ "/rootfs/root","/rootfs/path","/ssl" ]
ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT ["/run/entrypoint.sh"]