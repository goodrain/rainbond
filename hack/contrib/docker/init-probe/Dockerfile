FROM goodrainapps/alpine:3.4
COPY . /run/
RUN chmod 655 /run/rainbond-init-probe /run/entrypoint.sh
ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT [ "/run/entrypoint.sh" ]
CMD ["probe"]

