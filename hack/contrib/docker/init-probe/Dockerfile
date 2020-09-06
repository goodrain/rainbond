FROM goodrainapps/alpine:3.4
COPY . /run/
RUN chmod 655 /run/rainbond-init-probe /run/entrypoint.sh
ENV RELEASE_DESC=__RELEASE_DESC__
ENTRYPOINT [ "/run/entrypoint.sh" ]
CMD ["probe"]

