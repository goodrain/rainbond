FROM goodrainapps/alpine:3.4
ARG RELEASE_DESC
LABEL author="zengqg@goodrain.com"

ENV WORK_DIR=/run

RUN apk --no-cache add openssl openssh-client subversion
COPY rainbond-chaos entrypoint.sh /run/
COPY export-app /src/export-app

WORKDIR $WORK_DIR

ENV RELEASE_DESC=${RELEASE_DESC}

ENTRYPOINT ["/run/entrypoint.sh"]
