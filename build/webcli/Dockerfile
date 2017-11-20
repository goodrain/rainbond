FROM goodrainapps/alpine:3.4

ENV PORT 7070

ADD rainbond_webcli /usr/bin/rainbond_webcli

RUN mkdir /root/.kube

ENV RELEASE_DESC=__RELEASE_DESC__
ENTRYPOINT ["/usr/bin/rainbond_webcli"]