FROM goodrainapps/alpine:3.4
ADD . /run
ENV RELEASE_DESC=${RELEASE_DESC}
EXPOSE 6100 6101 9125
ENTRYPOINT ["/run/entrypoint.sh"]
