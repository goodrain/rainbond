FROM goodrainapps/alpine:3.4

LABEL author="zengqg@goodrain.com"

ENV WORK_DIR=/run

RUN apk --no-cache add openssl openssh-client subversion
COPY rainbond-chaos entrypoint.sh /run/
COPY export-app /src/export-app
COPY build-app /src/build-app

WORKDIR $WORK_DIR

ENV RELEASE_DESC=__RELEASE_DESC__

ENTRYPOINT ["/run/entrypoint.sh"]
