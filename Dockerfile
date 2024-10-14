FROM ubuntu:latest
LABEL authors="yangk"

ENTRYPOINT ["top", "-b"]