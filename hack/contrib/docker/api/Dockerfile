ARG BASE_IMAGE_VERSION
FROM debian:bullseye-slim
ARG RELEASE_DESC

ADD rainbond-api /run/rainbond-api
ADD entrypoint.sh /run/entrypoint.sh
WORKDIR /run
ENV RELEASE_DESC=${RELEASE_DESC}
VOLUME [ "/etc/goodrain" ]
ENTRYPOINT ["/run/entrypoint.sh"]