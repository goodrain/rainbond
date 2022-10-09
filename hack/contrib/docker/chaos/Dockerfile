ARG BASE_IMAGE_VERSION
FROM debian:bullseye-slim
ARG RELEASE_DESC


ENV WORK_DIR=/run

RUN apt update \
    && apt install openssl openssh-client subversion -y 
COPY rainbond-chaos entrypoint.sh /run/
COPY export-app /src/export-app

WORKDIR $WORK_DIR

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
