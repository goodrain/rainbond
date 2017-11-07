FROM goodrainapps/alpine:3.4

ADD rainbond_entrance /rainbond_entrance
VOLUME [ "/etc/goodrain" ]
ADD dist /api/dist

EXPOSE 8080

CMD /rainbond_entrance

