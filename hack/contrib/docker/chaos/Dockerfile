FROM goodrainapps/alpine:3.4

MAINTAINER zengqg@goodrain.com

RUN apk --no-cache add  openssl git openssh-client perl sudo

ADD rainbond-chaos /run/rainbond-chaos
ADD entrypoint.sh /run/entrypoint.sh
ADD build.pl /run/build.pl

ENV WORK_DIR /run

ENV RELEASE_DESC=__RELEASE_DESC__

WORKDIR $WORK_DIR

ENTRYPOINT ["/run/entrypoint.sh"]
