FROM goodrainapps/alpine:3.4
ARG RELEASE_DESC
ENV PORT 7070

ADD rainbond-webcli /usr/bin/rainbond-webcli
ADD entrypoint.sh /entrypoint.sh
RUN mkdir /root/.kube

ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT ["/entrypoint.sh"]