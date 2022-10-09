ARG BASE_IMAGE_VERSION
FROM debian:bullseye-slim
ARG RELEASE_DESC
ENV PORT 7070

ADD rainbond-webcli /usr/bin/rainbond-webcli
ADD entrypoint.sh /entrypoint.sh
RUN mkdir /root/.kube

ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT ["/entrypoint.sh"]