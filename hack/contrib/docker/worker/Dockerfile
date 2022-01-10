ARG BASE_IMAGE_VERSION
FROM goodrainapps/alpine:${BASE_IMAGE_VERSION}
ARG RELEASE_DESC
COPY rainbond-worker /run/rainbond-worker
COPY entrypoint.sh /run/entrypoint.sh
RUN chmod 655 /run/rainbond-worker

ENV EX_DOMAIN=ali-sh.goodrain.net:10080
ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
