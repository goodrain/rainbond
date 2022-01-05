ARG BASE_IMAGE_VERSION
FROM goodrainapps/alpine:${BASE_IMAGE_VERSION}
ARG RELEASE_DESC


ENV WORK_DIR=/run

RUN apk --no-cache add openssl openssh-client subversion
COPY rainbond-chaos entrypoint.sh /run/
COPY export-app /src/export-app

WORKDIR $WORK_DIR

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
