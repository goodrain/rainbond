FROM goodrainapps/alpine:3.4

ENV PORT 7070

ADD rainbond-webcli /usr/bin/rainbond-webcli
ADD kubectl /usr/bin/kubectl
ADD entrypoint.sh /entrypoint.sh
RUN mkdir /root/.kube

ENV RELEASE_DESC=__RELEASE_DESC__
ENTRYPOINT ["/entrypoint.sh"]